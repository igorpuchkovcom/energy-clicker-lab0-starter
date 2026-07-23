package mirrorscout

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestScanAllPreservesOrder(t *testing.T) {
	t.Parallel()

	candidates := make([]Candidate, 25)
	for i := range candidates {
		candidates[i] = Candidate{
			ID:  fmt.Sprintf("candidate-%02d", i),
			URL: "http://127.0.0.1:1",
		}
	}

	scanner := NewScanner(50 * time.Millisecond)
	results, err := scanner.ScanAll(context.Background(), candidates, 4)
	if err != nil {
		t.Fatalf("ScanAll() error = %v", err)
	}
	if len(results) != len(candidates) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(candidates))
	}
	for i := range candidates {
		if results[i].CandidateID != candidates[i].ID {
			t.Fatalf("results[%d].CandidateID = %q, want %q", i, results[i].CandidateID, candidates[i].ID)
		}
	}
}
