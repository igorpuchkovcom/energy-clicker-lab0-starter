package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/igor/energy-clicker/internal/store"
)

type fakeStore struct {
	collectPoints   int64
	collectReplayed bool
	collectKey      string
	pingErr         error
}

func (f *fakeStore) CreateSession(context.Context) (store.State, error) {
	return store.State{SessionID: "00000000-0000-4000-8000-000000000001"}, nil
}
func (f *fakeStore) GetState(context.Context, string) (store.State, error) {
	return store.State{SessionID: "00000000-0000-4000-8000-000000000001", Points: 4}, nil
}
func (f *fakeStore) CollectUnsafe(context.Context, string) (int64, error) {
	return 5, nil
}
func (f *fakeStore) Collect(_ context.Context, _, key string) (int64, bool, error) {
	f.collectKey = key
	return f.collectPoints, f.collectReplayed, nil
}
func (f *fakeStore) Ping(context.Context) error { return f.pingErr }
func (f *fakeStore) Close()                     {}

func newTestServer(t *testing.T, st store.Store) *Server {
	t.Helper()
	server, err := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), true)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return server
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, &fakeStore{})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestReadyzReturnsUnavailableDuringShutdown(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, &fakeStore{})
	server.SetShuttingDown()

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestCollectRequiresIdempotencyKey(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, &fakeStore{})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/collect",
		strings.NewReader(`{"session_id":"00000000-0000-4000-8000-000000000001"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestCollectPassesIdempotencyKey(t *testing.T) {
	t.Parallel()
	st := &fakeStore{collectPoints: 8, collectReplayed: true}
	server := newTestServer(t, st)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/collect",
		strings.NewReader(`{"session_id":"00000000-0000-4000-8000-000000000001"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "click-123")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if st.collectKey != "click-123" {
		t.Fatalf("key = %q, want click-123", st.collectKey)
	}
}
