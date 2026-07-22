package idgen

import (
	"regexp"
	"testing"
)

func TestUUID(t *testing.T) {
	t.Parallel()

	value, err := UUID()
	if err != nil {
		t.Fatalf("UUID() error = %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(value) {
		t.Fatalf("UUID() = %q, expected RFC 4122 v4 shape", value)
	}
}
