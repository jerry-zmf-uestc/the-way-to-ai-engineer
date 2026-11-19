package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

func uniqueSummaryJSON(dec *json.Decoder, depth int) error {
	if depth > 4 {
		return ErrSummary
	}
	token, err := dec.Token()
	if err != nil {
		return ErrSummary
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrSummary
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			key, err := dec.Token()
			if err != nil {
				return ErrSummary
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrSummary
			}
			seen[name] = true
		}
		if err := uniqueSummaryJSON(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrSummary
	}
	return nil
}

func ParseSummary(raw []byte) (Snapshot, error) {
	if len(raw) == 0 || len(raw) > 64<<10 || !utf8.Valid(raw) {
		return Snapshot{}, ErrSummary
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueSummaryJSON(dec, 0); err != nil {
		return Snapshot{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return Snapshot{}, ErrSummary
	}
	// Exact key sets reject encoding/json's case-insensitive aliases too.
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || len(root) != 2 || root["Version"] == nil || root["Facts"] == nil {
		return Snapshot{}, ErrSummary
	}
	var facts []map[string]json.RawMessage
	if json.Unmarshal(root["Facts"], &facts) != nil || len(facts) == 0 || len(facts) > 32 {
		return Snapshot{}, ErrSummary
	}
	for _, fields := range facts {
		if len(fields) != 4 {
			return Snapshot{}, ErrSummary
		}
		for _, key := range []string{"Key", "Value", "Source", "State"} {
			var value string
			if fields[key] == nil || bytes.Equal(bytes.TrimSpace(fields[key]), []byte("null")) || json.Unmarshal(fields[key], &value) != nil {
				return Snapshot{}, ErrSummary
			}
		}
	}
	var version string
	if bytes.Equal(bytes.TrimSpace(root["Version"]), []byte("null")) || json.Unmarshal(root["Version"], &version) != nil {
		return Snapshot{}, ErrSummary
	}
	var s Snapshot
	if json.Unmarshal(raw, &s) != nil {
		return Snapshot{}, ErrSummary
	}
	if _, err := factSet(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func inspectCandidate(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return err
	}
	candidate, parseErr := ParseSummary(raw)
	var accepted Snapshot
	if parseErr == nil {
		accepted, err = ValidateSummary(context.Background(), summaryExample(), candidate)
	}
	stage := "preserved"
	if parseErr != nil {
		stage = "invalid_shape"
	} else if err != nil {
		stage = "lost_or_changed_facts"
	}
	fmt.Printf("stage=%s accepted=%t published_facts=%d\n", stage, stage == "preserved", len(accepted.Facts))
	return nil
}
