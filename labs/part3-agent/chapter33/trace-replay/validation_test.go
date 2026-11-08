package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestContradictoryKindAndErrorCodeAreRejected(t *testing.T) {
	for _, kind := range []string{"model", "tool"} {
		code := "model_failed"
		if kind == "model" {
			code = "tool_failed"
		}
		archive := Archive{Schema: 1, Run: "r", Calls: []RecordedCall{{Kind: kind, Name: "fixed", InputHash: digest("q"), ErrorCode: code}}}
		data, _ := json.Marshal(archive)
		if _, err := Load(data); !errors.Is(err, ErrTrace) {
			t.Fatal(kind, code, err)
		}
		events := []Event{{Seq: 1, Run: "r", Step: "s", Call: "c", Kind: kind, Name: "fixed", Phase: "started", InputHash: digest("q")}, {Seq: 2, Run: "r", Step: "s", Call: "c", Kind: kind, Name: "fixed", Phase: "finished", InputHash: digest("q"), OutputHash: digest(""), ErrorCode: code}}
		if err := ValidateEvents(events); !errors.Is(err, ErrTrace) {
			t.Fatal("event contradiction", err)
		}
		for _, code := range []string{"", "canceled", "deadline", "protocol_failed"} {
			if !validKindCode(kind, code) {
				t.Fatal("shared category rejected", kind, code)
			}
		}
	}
}
func TestNegativeReplayCursorCannotPanicOrConsume(t *testing.T) {
	_, archive := recording(t)
	r := Replay{Archive: archive, Cursor: -1}
	if _, err := r.Do(context.Background(), "model", "decide", "question"); !errors.Is(err, ErrMismatch) || r.Cursor != -1 {
		t.Fatal(err, r.Cursor)
	}
}
