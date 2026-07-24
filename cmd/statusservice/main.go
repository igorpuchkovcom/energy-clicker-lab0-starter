package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	listenAddress  = ":8080"
	warmupDuration = 2 * time.Second
	shutdownPeriod = 5 * time.Second
	maxEchoBody    = 1 << 20 // 1 MiB
)

type App struct {
	startedAt  time.Time
	ready      atomic.Bool
	requestSeq atomic.Uint64
}

type statusResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`
	RequestID string `json:"request_id"`
}

type echoRequest struct {
	Message string `json:"message"`
	DelayMS int    `json:"delay_ms,omitempty"`
}

type echoResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

type requestIDContextKey struct{}

func main() {
	if err := run(); err != nil {
		log.Println("service error:", err)
		os.Exit(1)
	}
}

func run() error {
	app := &App{
		startedAt: time.Now(),
	}

	// Корневой context приложения отменяется по Ctrl+C или SIGTERM.
	appContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	go app.warmUp(appContext, warmupDuration)

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           app.routes(),
		ReadHeaderTimeout: 3 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Println("status service listening on", listenAddress)

		err := server.ListenAndServe()

		serverErrors <- err
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("HTTP server failed: %w", err)

	case <-appContext.Done():
		log.Println("shutdown requested:", appContext.Err())
	}

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		shutdownPeriod,
	)
	defer cancelShutdown()

	log.Println("waiting for active requests")

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Println("graceful shutdown failed:", err)
		log.Println("forcing server close")

		if closeErr := server.Close(); closeErr != nil {
			return fmt.Errorf(
				"force server close: %w",
				closeErr,
			)
		}
	}

	log.Println("server stopped")
	return nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", a.handleHealth)
	mux.HandleFunc("/readyz", a.handleReady)
	mux.HandleFunc("/echo", a.handleEcho)

	var handler http.Handler = mux

	handler = a.loggingMiddleware(handler)
	handler = a.requestIDMiddleware(handler)

	return handler
}

func (a *App) warmUp(
	ctx context.Context,
	duration time.Duration,
) {
	log.Println("service warming up for", duration)

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		a.ready.Store(true)
		log.Println("service is ready")

	case <-ctx.Done():
		log.Println("warmup cancelled")
	}
}

func (a *App) handleHealth(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r, http.MethodGet)
		return
	}

	response := statusResponse{
		Status:    "alive",
		Uptime:    time.Since(a.startedAt).Round(time.Millisecond).String(),
		RequestID: requestIDFromContext(r.Context()),
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleReady(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r, http.MethodGet)
		return
	}

	response := statusResponse{
		RequestID: requestIDFromContext(r.Context()),
	}

	if !a.ready.Load() {
		response.Status = "not_ready"

		writeJSON(
			w,
			http.StatusServiceUnavailable,
			response,
		)
		return
	}

	response.Status = "ready"

	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleEcho(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r, http.MethodPost)
		return
	}

	defer r.Body.Close()

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxEchoBody,
	)

	var input echoRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid JSON: "+err.Error(),
		)
		return
	}

	// Проверяем, что после первого JSON-объекта
	// в body нет второго значения.
	if err := ensureJSONFinished(decoder); err != nil {
		writeError(
			w,
			r,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if input.Message == "" {
		writeError(
			w,
			r,
			http.StatusBadRequest,
			"message is required",
		)
		return
	}

	if input.DelayMS < 0 || input.DelayMS > 10_000 {
		writeError(
			w,
			r,
			http.StatusBadRequest,
			"delay_ms must be between 0 and 10000",
		)
		return
	}

	if input.DelayMS > 0 {
		delay := time.Duration(input.DelayMS) *
			time.Millisecond

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Продолжаем после искусственной задержки.

		case <-r.Context().Done():
			log.Printf(
				"request %s cancelled during echo delay: %v",
				requestIDFromContext(r.Context()),
				r.Context().Err(),
			)
			return
		}
	}

	response := echoResponse{
		Message:   input.Message,
		RequestID: requestIDFromContext(r.Context()),
	}

	writeJSON(w, http.StatusOK, response)
}

func ensureJSONFinished(
	decoder *json.Decoder,
) error {
	var extra any

	err := decoder.Decode(&extra)

	switch {
	case errors.Is(err, io.EOF):
		return nil

	case err == nil:
		return errors.New(
			"request body must contain one JSON object",
		)

	default:
		return fmt.Errorf(
			"read remaining JSON: %w",
			err,
		)
	}
}

func methodNotAllowed(
	w http.ResponseWriter,
	r *http.Request,
	allowedMethod string,
) {
	w.Header().Set("Allow", allowedMethod)

	writeError(
		w,
		r,
		http.StatusMethodNotAllowed,
		"method not allowed",
	)
}

func writeError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	message string,
) {
	writeJSON(
		w,
		status,
		errorResponse{
			Error:     message,
			RequestID: requestIDFromContext(r.Context()),
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Println("encode response:", err)
	}
}

func (a *App) requestIDMiddleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			sequence := a.requestSeq.Add(1)

			requestID := "req-" +
				strconv.FormatUint(sequence, 10)

			ctx := context.WithValue(
				r.Context(),
				requestIDContextKey{},
				requestID,
			)

			w.Header().Set(
				"X-Request-ID",
				requestID,
			)

			next.ServeHTTP(
				w,
				r.WithContext(ctx),
			)
		},
	)
}

func (a *App) loggingMiddleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			started := time.Now()

			recorder := &statusRecorder{
				ResponseWriter: w,
			}

			next.ServeHTTP(recorder, r)

			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}

			log.Printf(
				"request_id=%s method=%s path=%s status=%d bytes=%d duration=%s",
				requestIDFromContext(r.Context()),
				r.Method,
				r.URL.Path,
				status,
				recorder.bytes,
				time.Since(started).Round(
					time.Microsecond,
				),
			)
		},
	)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(
	status int,
) {
	if r.status != 0 {
		return
	}

	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(
	data []byte,
) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}

	written, err := r.ResponseWriter.Write(data)
	r.bytes += written

	return written, err
}

func requestIDFromContext(
	ctx context.Context,
) string {
	requestID, ok := ctx.Value(
		requestIDContextKey{},
	).(string)

	if !ok {
		return ""
	}

	return requestID
}
