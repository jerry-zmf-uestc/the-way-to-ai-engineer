package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckpointRejectsAmbiguousJSONBeforeExecution(t *testing.T) {
	state := State{Schema: 2, Identity: id(), PlanHash: fingerprint(actions), Phase: "pending", MaxAttempts: 8}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)
	for name, raw := range map[string]string{
		"duplicate":    strings.TrimSuffix(base, "}") + `,"MaxAttempts":8}`,
		"alias":        strings.TrimSuffix(base, "}") + `,"maxattempts":8}`,
		"escaped-key":  strings.TrimSuffix(base, "}") + `,"Max\u0041ttempts":8}`,
		"nested":       strings.Replace(base, `"Tenant":`, `"Tenant":"ignored","tenant":`, 1),
		"invalid-utf8": strings.Replace(base, `"Results":null`, "\"Results\":[{\"Action\":\"read-A\",\"Text\":\"\xff\"}]", 1),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "checkpoint.json"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			store := open(t, dir)
			if _, err := store.Load(); !errors.Is(err, ErrState) {
				t.Fatalf("ambiguous payload loaded: %v", err)
			}
			_, err := Run(context.Background(), store, id(), actions, 8, true, func(context.Context, string) (string, error) {
				t.Fatal("ambiguous state reached tool")
				return "", nil
			}, nil)
			if !errors.Is(err, ErrState) {
				t.Fatal(err)
			}
		})
	}
}

func TestStateJSONBoundsAndLegitimateText(t *testing.T) {
	for _, raw := range []string{
		strings.Repeat("[", 10) + "0" + strings.Repeat("]", 10),
		`{"中":1}`, `{"a":[1,]}`, `{"a":1,"a":2}`, `{"a":{"B":1,"b":2}}`,
	} {
		if err := uniqueStateJSON(json.NewDecoder(strings.NewReader(raw)), 0); !errors.Is(err, ErrState) {
			t.Fatal("invalid token structure accepted", raw, err)
		}
	}
	store := open(t, t.TempDir())
	state := State{Schema: 2, Identity: id(), PlanHash: fingerprint(actions[:1]), Phase: "completed", Cursor: 1, Attempts: 1, MaxAttempts: 8, Results: []Result{{Action: actions[0], Text: "中文、引号\"、斜线\\与�"}}}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Results[0].Text != state.Results[0].Text {
		t.Fatal(loaded, err)
	}
	if _, err := Run(context.Background(), store, id(), actions[:1], 8, true, func(context.Context, string) (string, error) {
		t.Fatal("completed result was re-executed")
		return "", nil
	}, nil); err != nil {
		t.Fatal(err)
	}
}
