package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/event"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
)

// fakeTaskService 是 TaskService 的测试替身，精确控制返回值以验证错误映射。
type fakeTaskService struct {
	submitTask *task.Task
	submitErr  error
	getTask    *task.Task
	getErr     error
	cancelErr  error
}

func (f *fakeTaskService) Create(
	agentID string,
	sessionID string,
	input string,
) (*task.Task, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTaskService) Get(
	taskID string,
) (*task.Task, error) {
	return f.getTask, f.getErr
}

func (f *fakeTaskService) Events(
	taskID string,
) (<-chan event.Event, error) {
	return make(chan event.Event), nil
}

func (f *fakeTaskService) Execute(
	ctx context.Context,
	taskID string,
) (*task.Task, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTaskService) Submit(
	ctx context.Context,
	agentID string,
	sessionID string,
	input string,
) (*task.Task, error) {
	return f.submitTask, f.submitErr
}

func (f *fakeTaskService) Cancel(
	taskID string,
) error {
	return f.cancelErr
}

func newTestServer(fake *fakeTaskService) *Server {
	return NewServer(fake, session.NewManager())
}

func doRequest(
	t *testing.T,
	s *Server,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {

	t.Helper()

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	return rec
}

func TestCreateSession(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequest(
		t,
		s,
		http.MethodPost,
		"/v1/sessions",
		`{"agent_id":"demo-agent","user_id":"alice"}`,
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["session_id"] == "" {
		t.Fatal("expected session_id in response")
	}
	if resp["agent_id"] != "demo-agent" {
		t.Fatalf("expected agent_id demo-agent, got %q", resp["agent_id"])
	}
}

func TestCreateSessionMissingAgentID(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequest(
		t,
		s,
		http.MethodPost,
		"/v1/sessions",
		`{"user_id":"alice"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSubmitTask(t *testing.T) {
	fake := &fakeTaskService{
		submitTask: &task.Task{ID: "t1", Status: task.StatusPending},
	}
	s := newTestServer(fake)

	rec := doRequest(
		t,
		s,
		http.MethodPost,
		"/v1/tasks",
		`{"agent_id":"demo-agent","session_id":"s1","input":"现在几点了"}`,
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp task.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != "t1" {
		t.Fatalf("expected id t1, got %q", resp.ID)
	}
}

func TestSubmitTaskQueueFull(t *testing.T) {
	fake := &fakeTaskService{submitErr: task.ErrQueueFull}
	s := newTestServer(fake)

	rec := doRequest(
		t,
		s,
		http.MethodPost,
		"/v1/tasks",
		`{"agent_id":"a","session_id":"s1","input":"x"}`,
	)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestSubmitTaskValidationError(t *testing.T) {
	fake := &fakeTaskService{submitErr: errors.New("agent ID is empty")}
	s := newTestServer(fake)

	rec := doRequest(
		t,
		s,
		http.MethodPost,
		"/v1/tasks",
		`{"session_id":"s1","input":"x"}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetTask(t *testing.T) {
	fake := &fakeTaskService{
		getTask: &task.Task{
			ID:     "t1",
			Status: task.StatusCompleted,
			Output: "done",
		},
	}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodGet, "/v1/tasks/t1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp task.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Output != "done" {
		t.Fatalf("expected output done, got %q", resp.Output)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	fake := &fakeTaskService{getErr: task.ErrTaskNotFound}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodGet, "/v1/tasks/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCancelTask(t *testing.T) {
	fake := &fakeTaskService{}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodPost, "/v1/tasks/t1/cancel", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCancelTaskNotFound(t *testing.T) {
	fake := &fakeTaskService{cancelErr: task.ErrTaskNotFound}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodPost, "/v1/tasks/nope/cancel", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCancelTaskConflict(t *testing.T) {
	fake := &fakeTaskService{cancelErr: errors.New("task is not running")}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodPost, "/v1/tasks/t1/cancel", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}
