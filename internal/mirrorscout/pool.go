package mirrorscout

import (
	"context"
	"time"
)

func (s *Scanner) ScanAll(
	ctx context.Context,
	candidates []Candidate,
	concurrency int,
) ([]Result, error) {
	// TODO Lab 1 Milestone E:
	// - validate concurrency;
	// - start no more than concurrency workers;
	// - preserve input ordering in the returned slice;
	// - stop queueing and active work when ctx is cancelled;
	// - do not start one goroutine per candidate.
	_ = time.Second
	return nil, ErrNotImplemented
}
