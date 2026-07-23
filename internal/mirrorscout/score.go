package mirrorscout

func Score(checks Checks, totalDurationMS int64) (score int, healthy bool) {
	if checks.DNS.Status == StatusPassed {
		score += 15
	}
	if checks.TCP.Status == StatusPassed {
		score += 20
	}
	if checks.Health.Status == StatusPassed {
		score += 25
	}
	if checks.Readiness.Status == StatusPassed {
		score += 30
	}

	switch {
	case totalDurationMS <= 200:
		score += 10
	case totalDurationMS <= 500:
		score += 5
	}

	healthy = checks.DNS.Status == StatusPassed &&
		checks.TCP.Status == StatusPassed &&
		checks.Health.Status == StatusPassed &&
		checks.Readiness.Status == StatusPassed &&
		score >= 90

	return score, healthy
}
