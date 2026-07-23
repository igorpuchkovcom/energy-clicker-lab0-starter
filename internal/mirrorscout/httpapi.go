package mirrorscout

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type HTTPAPI struct {
	logger             *slog.Logger
	defaultConcurrency int
	defaultTimeout     time.Duration
}

func NewHTTPAPI(logger *slog.Logger, concurrency int, timeout time.Duration) *HTTPAPI {
	return &HTTPAPI{
		logger:             logger,
		defaultConcurrency: concurrency,
		defaultTimeout:     timeout,
	}
}

func (api *HTTPAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/scan", api.scan)
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	return mux
}

func (api *HTTPAPI) scan(w http.ResponseWriter, r *http.Request) {
	var request ScanRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(request.Candidates) == 0 {
		writeProblem(w, http.StatusBadRequest, "at least one candidate is required")
		return
	}

	concurrency := request.Concurrency
	if concurrency == 0 {
		concurrency = api.defaultConcurrency
	}
	if err := validateConcurrency(concurrency); err != nil {
		writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}

	timeout := api.defaultTimeout
	if request.TimeoutMS > 0 {
		timeout = time.Duration(request.TimeoutMS) * time.Millisecond
	}

	started := time.Now()
	scanner := NewScanner(timeout)
	results, err := scanner.ScanAll(r.Context(), request.Candidates, concurrency)

	response := summarize(results, time.Since(started), errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
	status := http.StatusOK
	if err != nil && !response.Cancelled {
		api.logger.ErrorContext(r.Context(), "scan failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, err.Error())
		return
	}
	if response.Cancelled {
		status = http.StatusRequestTimeout
	}
	writeJSON(w, status, response)
}

func summarize(results []Result, duration time.Duration, cancelled bool) ScanResponse {
	response := ScanResponse{
		Results:    results,
		Total:      len(results),
		Cancelled:  cancelled,
		DurationMS: duration.Milliseconds(),
	}
	for _, result := range results {
		if result.Healthy {
			response.Healthy++
		} else {
			response.Unhealthy++
		}
	}
	return response
}

func (api *HTTPAPI) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (api *HTTPAPI) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
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
