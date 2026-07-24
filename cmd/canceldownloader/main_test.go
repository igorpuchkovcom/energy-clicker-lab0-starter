package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDownloadTimeout(t *testing.T) {
	server := newBlockingServer()
	defer server.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		50*time.Millisecond,
	)
	defer cancel()

	result := download(
		ctx,
		server.Client(),
		Job{
			Name: "timeout",
			URL:  server.URL,
		},
	)

	if !errors.Is(
		result.Err,
		context.DeadlineExceeded,
	) {
		t.Fatalf(
			"error = %v, want deadline exceeded",
			result.Err,
		)
	}
}

func TestDownloadCancellation(t *testing.T) {
	server := newBlockingServer()
	defer server.Close()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := download(
		ctx,
		server.Client(),
		Job{
			Name: "cancelled",
			URL:  server.URL,
		},
	)

	if !errors.Is(
		result.Err,
		context.Canceled,
	) {
		t.Fatalf(
			"error = %v, want context canceled",
			result.Err,
		)
	}
}

func newBlockingServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				<-r.Context().Done()
			},
		),
	)
}
