package mirrorscout

import (
	"encoding/json"
	"fmt"
	"io"
)

func DecodeCandidates(r io.Reader) ([]Candidate, error) {
	var candidates []Candidate
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidates); err != nil {
		return nil, fmt.Errorf("decode candidates: %w", err)
	}
	for i, candidate := range candidates {
		if candidate.ID == "" {
			return nil, fmt.Errorf("candidate %d: id is required", i)
		}
		if candidate.URL == "" {
			return nil, fmt.Errorf("candidate %q: url is required", candidate.ID)
		}
	}
	return candidates, nil
}
