package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
	"github.com/google/uuid"
)

// Server 是 Agent Runtime 的 HTTP 传输层，对外暴露任务与会话接口。
type Server struct {
	tasks    task.TaskService
	sessions *session.Manager
}

func NewServer(
	tasks task.TaskService,
	sessions *session.Manager,
) *Server {
	return &Server{
		tasks:    tasks,
		sessions: sessions,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	mux.HandleFunc("POST /v1/tasks", s.handleSubmit)
	mux.HandleFunc("GET /v1/tasks/{id}", s.handleGet)
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", s.handleCancel)

	return mux
}

type createSessionRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	UserID    string `json:"user_id"`
}

func (s *Server) handleCreateSession(
	w http.ResponseWriter,
	r *http.Request,
) {

	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if req.SessionID == "" {
		req.SessionID = uuid.NewString()
	}

	sess, err := s.sessions.Create(
		req.SessionID,
		req.AgentID,
		req.UserID,
	)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id": sess.ID,
		"agent_id":   sess.AgentID,
	})
}

type submitRequest struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
	Input     string `json:"input"`
}

func (s *Server) handleSubmit(
	w http.ResponseWriter,
	r *http.Request,
) {

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t, err := s.tasks.Submit(
		r.Context(),
		req.AgentID,
		req.SessionID,
		req.Input,
	)
	if err != nil {
		if errors.Is(err, task.ErrQueueFull) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, t)
}

func (s *Server) handleGet(
	w http.ResponseWriter,
	r *http.Request,
) {

	id := r.PathValue("id")

	t, err := s.tasks.Get(id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleCancel(
	w http.ResponseWriter,
	r *http.Request,
) {

	id := r.PathValue("id")

	if err := s.tasks.Cancel(id); err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "canceled",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
