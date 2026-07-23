package mirrorscout

import (
	"context"
	"net/http"
	"time"
)

func scanCandidate(
	ctx context.Context,
	candidate Candidate,
	resolver Resolver,
	dialer Dialer,
	client HTTPDoer,
	timeout time.Duration,
) Result {
	// TODO Lab 1 Milestones A–D:
	// - parse the candidate URL;
	// - create a timeout context for the whole candidate;
	// - perform DNS, TCP, /healthz, and /readyz checks;
	// - skip dependent checks after DNS/TCP failures;
	// - measure each check and the total duration;
	// - calculate score and healthy.
	return Result{
		CandidateID: candidate.ID,
		URL:         candidate.URL,
		StartedAt:   time.Now().UTC(),
		Checks: Checks{
			DNS:       CheckResult{Status: StatusSkipped, Error: "TODO"},
			TCP:       CheckResult{Status: StatusSkipped, Error: "TODO"},
			Health:    CheckResult{Status: StatusSkipped, Error: "TODO"},
			Readiness: CheckResult{Status: StatusSkipped, Error: "TODO"},
		},
	}
}

func checkHTTP(
	ctx context.Context,
	client HTTPDoer,
	baseURL string,
	path string,
	expectedStatus int,
) CheckResult {
	// TODO Lab 1 Milestone C.
	_ = http.StatusOK
	return CheckResult{Status: StatusSkipped, Error: "TODO"}
}
