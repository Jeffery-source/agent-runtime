package transport

import (
	"encoding/json"
	"errors"
	"fmt"
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

const (
	// allowedOrigin 是允许跨域访问的来源（Web Console）。
	allowedOrigin = "http://localhost:5173"

	// allowedMethods 与 allowedHeaders 是 CORS 协商放行的方法与请求头。
	allowedMethods = "GET, POST, OPTIONS"
	allowedHeaders = "Content-Type"
)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)

	mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	mux.HandleFunc("POST /v1/tasks", s.handleSubmit)
	mux.HandleFunc("GET /v1/tasks", s.handleList)
	mux.HandleFunc("GET /v1/tasks/{id}", s.handleGet)
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", s.handleCancel)

	mux.HandleFunc("GET /v1/tasks/{id}/events", s.handleEvents)

	return corsMiddleware(mux)
}

// corsMiddleware 统一处理跨域请求，使 Web Console 可以访问本服务。
// CORS 收口在路由层，各业务 Handler 不需要关心跨域细节。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Header.Get("Origin") == allowedOrigin {
			w.Header().Set(
				"Access-Control-Allow-Origin",
				allowedOrigin,
			)
			w.Header().Set(
				"Access-Control-Allow-Methods",
				allowedMethods,
			)
			w.Header().Set(
				"Access-Control-Allow-Headers",
				allowedHeaders,
			)
			w.Header().Add("Vary", "Origin")
		}

		// preflight 请求不进入业务 Handler，直接返回 204。
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(
	w http.ResponseWriter,
	r *http.Request,
) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

type listTasksResponse struct {
	Items []*task.Task `json:"items"`
	Total int          `json:"total"`
}

func (s *Server) handleList(
	w http.ResponseWriter,
	r *http.Request,
) {
	items := s.tasks.List()

	// 空列表序列化为 []，避免前端拿到 null。
	if items == nil {
		items = []*task.Task{}
	}

	writeJSON(w, http.StatusOK, listTasksResponse{
		Items: items,
		Total: len(items),
	})
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

func (s *Server) handleEvents(
	w http.ResponseWriter,
	r *http.Request,
) {
	taskID := r.PathValue("id")

	events, err := s.tasks.Events(taskID)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}

		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(
			w,
			http.StatusInternalServerError,
			"streaming unsupported",
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"text/event-stream",
	)
	w.Header().Set(
		"Cache-Control",
		"no-cache",
	)
	w.Header().Set(
		"Connection",
		"keep-alive",
	)
	w.Header().Set(
		"X-Accel-Buffering",
		"no",
	)

	flusher.Flush()

	for {
		select {

		case <-r.Context().Done():
			return

		case e, ok := <-events:
			if !ok {
				return
			}

			data, err := json.Marshal(e.Data)
			if err != nil {
				continue
			}

			fmt.Fprintf(
				w,
				"event: %s\n",
				e.Type,
			)

			fmt.Fprintf(
				w,
				"data: %s\n\n",
				data,
			)

			flusher.Flush()
		}
	}
}
