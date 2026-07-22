package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/igor/energy-clicker/internal/store"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	store               store.Store
	logger              *slog.Logger
	allowDebugEndpoints bool
	shuttingDown        atomic.Bool
	staticHandler       http.Handler
}

func New(st store.Store, logger *slog.Logger, allowDebugEndpoints bool) (*Server, error) {
	webFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("create static filesystem: %w", err)
	}

	return &Server{
		store:               st,
		logger:              logger,
		allowDebugEndpoints: allowDebugEndpoints,
		staticHandler:       http.FileServer(http.FS(webFS)),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/session", s.createSession)
	mux.HandleFunc("GET /api/state/{sessionID}", s.getState)
	mux.HandleFunc("POST /api/collect", s.collect)
	mux.HandleFunc("POST /api/debug/collect-unsafe", s.collectUnsafe)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.Handle("/", s.staticHandler)

	return s.requestLogMiddleware(mux)
}

func (s *Server) SetShuttingDown() {
	s.shuttingDown.Store(true)
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	state, err := s.store.CreateSession(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, state)
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("sessionID"))
	if sessionID == "" {
		writeProblem(w, http.StatusBadRequest, "session_id is required")
		return
	}

	state, err := s.store.GetState(r.Context(), sessionID)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

type collectRequest struct {
	SessionID string `json:"session_id"`
}

type collectResponse struct {
	SessionID      string `json:"session_id"`
	Points         int64  `json:"points"`
	Replayed       bool   `json:"replayed"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func (s *Server) collect(w http.ResponseWriter, r *http.Request) {
	var input collectRequest
	if err := decodeJSON(r, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	input.SessionID = strings.TrimSpace(input.SessionID)
	if input.SessionID == "" {
		writeProblem(w, http.StatusBadRequest, "session_id is required")
		return
	}

	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeProblem(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}
	if len(key) > 200 {
		writeProblem(w, http.StatusBadRequest, "Idempotency-Key must be at most 200 characters")
		return
	}

	points, replayed, err := s.store.Collect(r.Context(), input.SessionID, key)
	if errors.Is(err, store.ErrNotImplemented) {
		writeProblem(w, http.StatusNotImplemented, "safe collect is your Lab 0 task")
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	s.delayAfterCommit(r)
	writeJSON(w, http.StatusOK, collectResponse{
		SessionID:      input.SessionID,
		Points:         points,
		Replayed:       replayed,
		IdempotencyKey: key,
	})
}

func (s *Server) collectUnsafe(w http.ResponseWriter, r *http.Request) {
	if !s.allowDebugEndpoints {
		writeProblem(w, http.StatusNotFound, "not found")
		return
	}

	var input collectRequest
	if err := decodeJSON(r, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	input.SessionID = strings.TrimSpace(input.SessionID)
	if input.SessionID == "" {
		writeProblem(w, http.StatusBadRequest, "session_id is required")
		return
	}

	points, err := s.store.CollectUnsafe(r.Context(), input.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	s.delayAfterCommit(r)
	writeJSON(w, http.StatusOK, collectResponse{
		SessionID: input.SessionID,
		Points:    points,
	})
}

func (s *Server) delayAfterCommit(r *http.Request) {
	if !s.allowDebugEndpoints {
		return
	}
	raw := strings.TrimSpace(r.Header.Get("X-Debug-Delay-After-Commit-Ms"))
	if raw == "" {
		return
	}
	milliseconds, err := strconv.Atoi(raw)
	if err != nil || milliseconds <= 0 {
		return
	}
	if milliseconds > 10_000 {
		milliseconds = 10_000
	}

	// Deliberately ignore request cancellation here. The database transaction
	// has already committed; this pause creates the "effect happened but the
	// response was lost" experiment.
	time.Sleep(time.Duration(milliseconds) * time.Millisecond)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.shuttingDown.Load() {
		writeProblem(w, http.StatusServiceUnavailable, "shutting down")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 750*time.Millisecond)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "request failed", "error", err)
	writeProblem(w, http.StatusInternalServerError, "internal server error")
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"status": status,
		"error":  message,
	})
}
