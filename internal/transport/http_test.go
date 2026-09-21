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
	listTasks  []*task.Task
	eventsCh   chan event.Event
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

func (f *fakeTaskService) List() []*task.Task {
	return f.listTasks
}

func (f *fakeTaskService) Events(
	taskID string,
) (<-chan event.Event, error) {
	if f.eventsCh != nil {
		return f.eventsCh, nil
	}
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

	return doRequestWithHeaders(t, s, method, path, body, nil)
}

func doRequestWithHeaders(
	t *testing.T,
	s *Server,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {

	t.Helper()

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	for k, v := range headers {
		req.Header.Set(k, v)
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

func TestHealthz(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequest(t, s, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", resp["status"])
	}
}

func TestCORSHeaders(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequestWithHeaders(
		t,
		s,
		http.MethodGet,
		"/healthz",
		"",
		map[string]string{"Origin": allowedOrigin},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("expected allow-origin %q, got %q", allowedOrigin, got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != allowedMethods {
		t.Fatalf("expected allow-methods %q, got %q", allowedMethods, got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != allowedHeaders {
		t.Fatalf("expected allow-headers %q, got %q", allowedHeaders, got)
	}
}

func TestCORSPreflight(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequestWithHeaders(
		t,
		s,
		http.MethodOptions,
		"/v1/tasks",
		"",
		map[string]string{
			"Origin":                         allowedOrigin,
			"Access-Control-Request-Method":  "POST",
			"Access-Control-Request-Headers": "Content-Type",
		},
	)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("expected allow-origin %q, got %q", allowedOrigin, got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestCORSIgnoresUnknownOrigin(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequestWithHeaders(
		t,
		s,
		http.MethodGet,
		"/healthz",
		"",
		map[string]string{"Origin": "http://evil.example.com"},
	)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header, got %q", got)
	}
}

func TestListTasks(t *testing.T) {
	fake := &fakeTaskService{
		listTasks: []*task.Task{
			{
				ID:        "t1",
				AgentID:   "demo-agent",
				SessionID: "s1",
				Input:     "现在几点了",
				Status:    task.StatusCompleted,
			},
			{
				ID:        "t2",
				AgentID:   "demo-agent",
				SessionID: "s1",
				Input:     "你好",
				Status:    task.StatusPending,
			},
		},
	}
	s := newTestServer(fake)

	rec := doRequest(t, s, http.MethodGet, "/v1/tasks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Items []task.Task `json:"items"`
		Total int         `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != "t1" || resp.Items[0].Input != "现在几点了" {
		t.Fatalf("unexpected first item: %+v", resp.Items[0])
	}
	if resp.Items[1].Status != task.StatusPending {
		t.Fatalf("expected pending, got %q", resp.Items[1].Status)
	}
}

func TestListTasksEmpty(t *testing.T) {
	s := newTestServer(&fakeTaskService{})

	rec := doRequest(t, s, http.MethodGet, "/v1/tasks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if string(raw["items"]) != "[]" {
		t.Fatalf("expected items to be [], got %s", raw["items"])
	}
	if string(raw["total"]) != "0" {
		t.Fatalf("expected total 0, got %s", raw["total"])
	}
}

// TestTaskEventsStream 保证 SSE 事件流在加入 CORS 中间件后依然可用。
func TestTaskEventsStream(t *testing.T) {
	ch := make(chan event.Event, 1)
	fake := &fakeTaskService{eventsCh: ch}
	s := newTestServer(fake)

	go func() {
		ch <- event.Event{
			Type: "status",
			Data: map[string]string{"status": "running"},
		}
		close(ch)
	}()

	rec := doRequest(t, s, http.MethodGet, "/v1/tasks/t1/events", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: status") {
		t.Fatalf("expected event line in body, got %q", body)
	}
	if !strings.Contains(body, `"status":"running"`) {
		t.Fatalf("expected event data in body, got %q", body)
	}
}
