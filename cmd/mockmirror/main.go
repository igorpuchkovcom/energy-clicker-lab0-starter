package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	address := getenv("MIRROR_ADDR", ":18081")
	mode := getenv("MIRROR_MODE", "good")
	delay := parseDuration(getenv("MIRROR_DELAY", "800ms"), 800*time.Millisecond)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, mode, "health", delay)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, mode, "readiness", delay)
	})

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("mock mirror started", "address", address, "mode", mode, "delay", delay)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("mock mirror shutdown requested")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("mock mirror failed", "error", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("mock mirror shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("mock mirror stopped")
}

func respond(w http.ResponseWriter, r *http.Request, mode, check string, delay time.Duration) {
	switch mode {
	case "hanging":
		select {
		case <-time.After(30 * time.Second):
		case <-r.Context().Done():
			return
		}
	case "slow":
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	case "health500":
		if check == "health" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "broken"})
			return
		}
	case "notready":
		if check == "readiness" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
	}

	status := "alive"
	if check == "readiness" {
		status = "ready"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
