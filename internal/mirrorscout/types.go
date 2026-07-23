package mirrorscout

import "time"

type Candidate struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type CheckStatus string

const (
	StatusPassed  CheckStatus = "passed"
	StatusFailed  CheckStatus = "failed"
	StatusSkipped CheckStatus = "skipped"
)

type CheckResult struct {
	Status    CheckStatus `json:"status"`
	LatencyMS int64       `json:"latency_ms"`
	Error     string      `json:"error,omitempty"`
	Detail    string      `json:"detail,omitempty"`
}

type Checks struct {
	DNS       CheckResult `json:"dns"`
	TCP       CheckResult `json:"tcp"`
	Health    CheckResult `json:"health"`
	Readiness CheckResult `json:"readiness"`
}

type Result struct {
	CandidateID string    `json:"candidate_id"`
	URL         string    `json:"url"`
	StartedAt   time.Time `json:"started_at"`
	DurationMS  int64     `json:"duration_ms"`
	Score       int       `json:"score"`
	Healthy     bool      `json:"healthy"`
	Checks      Checks    `json:"checks"`
}

type ScanRequest struct {
	Candidates  []Candidate `json:"candidates"`
	Concurrency int         `json:"concurrency,omitempty"`
	TimeoutMS   int         `json:"timeout_ms,omitempty"`
}

type ScanResponse struct {
	Results    []Result `json:"results"`
	Total      int      `json:"total"`
	Healthy    int      `json:"healthy"`
	Unhealthy  int      `json:"unhealthy"`
	Cancelled  bool     `json:"cancelled"`
	DurationMS int64    `json:"duration_ms"`
}
