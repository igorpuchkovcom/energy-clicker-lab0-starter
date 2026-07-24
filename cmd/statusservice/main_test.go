package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestApp(ready bool) *App {
	app := &App{
		startedAt: time.Now().Add(-time.Second),
	}

	app.ready.Store(ready)

	return app
}

func silenceLogs(t testing.TB) {
	originalWriter := log.Writer()

	log.SetOutput(io.Discard)

	t.Cleanup(func() {
		log.SetOutput(originalWriter)
	})
}

func TestHealthHandler(t *testing.T) {
	silenceLogs(t)

	app := newTestApp(true)
	handler := app.routes()

	request := httptest.NewRequest(
		http.MethodGet,
		"/healthz",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	var response statusResponse

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf(
			"decode response JSON: %v",
			err,
		)
	}

	if response.Status != "alive" {
		t.Errorf(
			"status = %q, want %q",
			response.Status,
			"alive",
		)
	}

	if response.Uptime == "" {
		t.Error("uptime is empty")
	}

	requestID := recorder.Header().Get(
		"X-Request-ID",
	)

	if requestID == "" {
		t.Fatal("X-Request-ID header is empty")
	}

	if response.RequestID != requestID {
		t.Errorf(
			"response request ID = %q, header = %q",
			response.RequestID,
			requestID,
		)
	}
}

func TestReadyHandler(t *testing.T) {
	tests := []struct {
		name       string
		ready      bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "service is not ready",
			ready:      false,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "not_ready",
		},
		{
			name:       "service is ready",
			ready:      true,
			wantStatus: http.StatusOK,
			wantBody:   "ready",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			silenceLogs(t)

			app := newTestApp(test.ready)
			handler := app.routes()

			request := httptest.NewRequest(
				http.MethodGet,
				"/readyz",
				nil,
			)

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					recorder.Code,
					test.wantStatus,
				)
			}

			var response statusResponse

			if err := json.Unmarshal(
				recorder.Body.Bytes(),
				&response,
			); err != nil {
				t.Fatalf(
					"decode response JSON: %v",
					err,
				)
			}

			if response.Status != test.wantBody {
				t.Errorf(
					"body status = %q, want %q",
					response.Status,
					test.wantBody,
				)
			}
		})
	}
}

func TestEchoHandler(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		body              string
		wantStatus        int
		wantMessage       string
		wantErrorContains string
	}{
		{
			name:        "valid request",
			method:      http.MethodPost,
			body:        `{"message":"hello"}`,
			wantStatus:  http.StatusOK,
			wantMessage: "hello",
		},
		{
			name:              "missing message",
			method:            http.MethodPost,
			body:              `{}`,
			wantStatus:        http.StatusBadRequest,
			wantErrorContains: "message is required",
		},
		{
			name:              "unknown JSON field",
			method:            http.MethodPost,
			body:              `{"message":"hello","unknown":true}`,
			wantStatus:        http.StatusBadRequest,
			wantErrorContains: "unknown field",
		},
		{
			name:              "malformed JSON",
			method:            http.MethodPost,
			body:              `{"message":}`,
			wantStatus:        http.StatusBadRequest,
			wantErrorContains: "invalid JSON",
		},
		{
			name:              "two JSON objects",
			method:            http.MethodPost,
			body:              `{"message":"one"} {"message":"two"}`,
			wantStatus:        http.StatusBadRequest,
			wantErrorContains: "one JSON object",
		},
		{
			name:              "wrong method",
			method:            http.MethodGet,
			body:              "",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "method not allowed",
		},
		{
			name:              "negative delay",
			method:            http.MethodPost,
			body:              `{"message":"hello","delay_ms":-1}`,
			wantStatus:        http.StatusBadRequest,
			wantErrorContains: "delay_ms",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			silenceLogs(t)

			app := newTestApp(true)
			handler := app.routes()

			request := httptest.NewRequest(
				test.method,
				"/echo",
				strings.NewReader(test.body),
			)

			if test.body != "" {
				request.Header.Set(
					"Content-Type",
					"application/json",
				)
			}

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					test.wantStatus,
					recorder.Body.String(),
				)
			}

			var response struct {
				Message   string `json:"message"`
				Error     string `json:"error"`
				RequestID string `json:"request_id"`
			}

			if err := json.Unmarshal(
				recorder.Body.Bytes(),
				&response,
			); err != nil {
				t.Fatalf(
					"decode response JSON: %v",
					err,
				)
			}

			if response.RequestID == "" {
				t.Error("response request ID is empty")
			}

			if test.wantMessage != "" &&
				response.Message != test.wantMessage {
				t.Errorf(
					"message = %q, want %q",
					response.Message,
					test.wantMessage,
				)
			}

			if test.wantErrorContains != "" &&
				!strings.Contains(
					response.Error,
					test.wantErrorContains,
				) {
				t.Errorf(
					"error = %q, want substring %q",
					response.Error,
					test.wantErrorContains,
				)
			}
		})
	}
}

func TestRequestIDsAreUniqueConcurrently(
	t *testing.T,
) {
	silenceLogs(t)

	app := newTestApp(true)
	handler := app.routes()

	const requestCount = 100

	ids := make(chan string, requestCount)
	errorsCh := make(chan error, requestCount)

	var wg sync.WaitGroup

	for i := 0; i < requestCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			request := httptest.NewRequest(
				http.MethodGet,
				"/healthz",
				nil,
			)

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(
				recorder,
				request,
			)

			if recorder.Code != http.StatusOK {
				errorsCh <- fmt.Errorf(
					"status = %d",
					recorder.Code,
				)
				return
			}

			requestID := recorder.Header().Get(
				"X-Request-ID",
			)

			if requestID == "" {
				errorsCh <- fmt.Errorf(
					"empty request ID",
				)
				return
			}

			ids <- requestID
		}()
	}

	wg.Wait()

	close(ids)
	close(errorsCh)

	for err := range errorsCh {
		t.Error(err)
	}

	uniqueIDs := make(map[string]struct{})

	for requestID := range ids {
		if _, exists := uniqueIDs[requestID]; exists {
			t.Errorf(
				"duplicate request ID: %s",
				requestID,
			)
		}

		uniqueIDs[requestID] = struct{}{}
	}

	if len(uniqueIDs) != requestCount {
		t.Errorf(
			"unique IDs = %d, want %d",
			len(uniqueIDs),
			requestCount,
		)
	}
}

func BenchmarkHealthHandler(b *testing.B) {
	silenceLogs(b)

	app := newTestApp(true)
	handler := app.routes()

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		request := httptest.NewRequest(
			http.MethodGet,
			"/healthz",
			nil,
		)

		recorder := httptest.NewRecorder()

		handler.ServeHTTP(
			recorder,
			request,
		)

		if recorder.Code != http.StatusOK {
			b.Fatalf(
				"status = %d, want %d",
				recorder.Code,
				http.StatusOK,
			)
		}
	}
}
