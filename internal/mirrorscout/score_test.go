package mirrorscout

import "testing"

func TestScore(t *testing.T) {
	t.Parallel()

	allPassed := Checks{
		DNS:       CheckResult{Status: StatusPassed},
		TCP:       CheckResult{Status: StatusPassed},
		Health:    CheckResult{Status: StatusPassed},
		Readiness: CheckResult{Status: StatusPassed},
	}

	tests := []struct {
		name       string
		checks     Checks
		durationMS int64
		wantScore  int
		wantHealth bool
	}{
		{
			name:       "fast healthy candidate",
			checks:     allPassed,
			durationMS: 100,
			wantScore:  100,
			wantHealth: true,
		},
		{
			name:       "slow but healthy candidate",
			checks:     allPassed,
			durationMS: 900,
			wantScore:  90,
			wantHealth: true,
		},
		{
			name: "not ready",
			checks: Checks{
				DNS:       CheckResult{Status: StatusPassed},
				TCP:       CheckResult{Status: StatusPassed},
				Health:    CheckResult{Status: StatusPassed},
				Readiness: CheckResult{Status: StatusFailed},
			},
			durationMS: 100,
			wantScore:  70,
			wantHealth: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			score, healthy := Score(tt.checks, tt.durationMS)
			if score != tt.wantScore {
				t.Fatalf("Score() score = %d, want %d", score, tt.wantScore)
			}
			if healthy != tt.wantHealth {
				t.Fatalf("Score() healthy = %v, want %v", healthy, tt.wantHealth)
			}
		})
	}
}
