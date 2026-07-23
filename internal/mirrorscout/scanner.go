package mirrorscout

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Scanner struct {
	resolver Resolver
	dialer   Dialer
	client   HTTPDoer
	timeout  time.Duration
}

func NewScanner(timeout time.Duration) *Scanner {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true

	return &Scanner{
		resolver: net.DefaultResolver,
		dialer: &net.Dialer{
			KeepAlive: -1,
		},
		client: &http.Client{
			Transport: transport,
		},
		timeout: timeout,
	}
}

func (s *Scanner) ScanCandidate(ctx context.Context, candidate Candidate) Result {
	return scanCandidate(ctx, candidate, s.resolver, s.dialer, s.client, s.timeout)
}

func validateConcurrency(concurrency int) error {
	if concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	if concurrency > 1000 {
		return fmt.Errorf("concurrency must not exceed 1000")
	}
	return nil
}
