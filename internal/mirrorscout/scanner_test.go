package mirrorscout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScanCandidateHealthy(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"alive"}`))
		case "/readyz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	scanner := NewScanner(time.Second)
	result := scanner.ScanCandidate(context.Background(), Candidate{
		ID:  "healthy",
		URL: server.URL,
	})

	if !result.Healthy {
		t.Fatalf("result.Healthy = false, result = %+v", result)
	}
	if result.Score < 90 {
		t.Fatalf("result.Score = %d, want at least 90", result.Score)
	}
}

func TestScanCandidateTimesOut(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	scanner := NewScanner(100 * time.Millisecond)
	started := time.Now()
	result := scanner.ScanCandidate(context.Background(), Candidate{
		ID:  "hanging",
		URL: server.URL,
	})

	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}
	if time.Since(started) > time.Second {
		t.Fatalf("scan did not respect timeout: %s", time.Since(started))
	}
}
