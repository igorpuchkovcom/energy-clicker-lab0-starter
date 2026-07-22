package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/igor/energy-clicker/internal/config"
	"github.com/igor/energy-clicker/internal/httpapi"
	postgresstore "github.com/igor/energy-clicker/internal/store/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()

	st, err := postgresstore.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("open PostgreSQL store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	api, err := httpapi.New(st, logger, cfg.AllowDebugEndpoints)
	if err != nil {
		logger.Error("create HTTP API", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server started", "address", cfg.Address)
		serveErr <- httpServer.ListenAndServe()
	}()

	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received", "cause", context.Cause(signalCtx))
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	api.SetShuttingDown()

	// Give a load balancer or readiness poller a brief opportunity to observe
	// the not-ready state before listeners close.
	time.Sleep(250 * time.Millisecond)

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown timed out", "error", err)
		_ = httpServer.Close()
		os.Exit(1)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
