package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/igor/energy-clicker/internal/mirrorscout"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "scan":
		if err := runScan(ctx, os.Args[2:]); err != nil {
			logger.Error("scan failed", "error", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(ctx, logger, os.Args[2:]); err != nil {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runScan(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	file := flags.String("file", "", "JSON file containing candidate mirrors")
	concurrency := flags.Int("concurrency", 20, "maximum concurrent candidate scans")
	timeout := flags.Duration("timeout", 2*time.Second, "timeout per candidate")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}

	f, err := os.Open(*file)
	if err != nil {
		return fmt.Errorf("open candidates file: %w", err)
	}
	defer f.Close()

	candidates, err := mirrorscout.DecodeCandidates(f)
	if err != nil {
		return err
	}

	scanner := mirrorscout.NewScanner(*timeout)
	results, scanErr := scanner.ScanAll(ctx, candidates, *concurrency)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("encode results: %w", err)
	}
	return scanErr
}

func runServe(ctx context.Context, logger *slog.Logger, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	address := flags.String("addr", ":8090", "HTTP listen address")
	concurrency := flags.Int("concurrency", 20, "default maximum concurrent candidate scans")
	timeout := flags.Duration("timeout", 2*time.Second, "default timeout per candidate")
	if err := flags.Parse(args); err != nil {
		return err
	}

	api := mirrorscout.NewHTTPAPI(logger, *concurrency, *timeout)
	server := &http.Server{
		Addr:              *address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("MirrorScout HTTP API started", "address", *address)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("MirrorScout shutdown requested")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  mirrorscout scan  --file configs/mirrors.local.json --concurrency 20 --timeout 2s
  mirrorscout serve --addr :8090 --concurrency 20 --timeout 2s`)
}
