package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestInvalidTextCannotBecomeSuccessfulRecording(t *testing.T) {
	r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
	text, err := r.Do(context.Background(), "model", "respond", "q", func(context.Context) (string, error) { return string([]byte{0xff}), nil })
	if text != "" || !errors.Is(err, ErrTrace) || r.Archive.Calls[0].ErrorCode != "protocol_failed" {
		t.Fatal("invalid encoding accepted", text, err, r.Archive)
	}
	data, err := json.Marshal(r.Archive)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	replay := Replay{Archive: a}
	if got, err := replay.Do(context.Background(), "model", "respond", "q"); got != "" || !errors.Is(err, ErrTrace) || replay.Done() != nil {
		t.Fatal("recorded rejection failed to replay", got, err)
	}
}

func TestInvalidTextInputRejectedBeforeLiveAndReplay(t *testing.T) {
	bad := string([]byte{0xff})
	for _, tc := range []struct{ run, name, input string }{{"r", "read", bad}, {"r", bad, "q"}, {bad, "read", "q"}} {
		r := Recorder{Archive: Archive{Schema: 1, Run: tc.run}}
		calls := 0
		if _, err := r.Do(context.Background(), "tool", tc.name, tc.input, func(context.Context) (string, error) { calls++; return "ok", nil }); !errors.Is(err, ErrTrace) || calls != 0 || len(r.Events) != 0 || len(r.Archive.Calls) != 0 {
			t.Fatal("invalid text reached live call", err, calls)
		}
	}
	replay := Replay{Archive: Archive{Schema: 1, Run: "r", Calls: []RecordedCall{{Kind: "tool", Name: "read", InputHash: digest(bad), Response: "ok"}}}}
	if _, err := replay.Do(context.Background(), "tool", "read", bad); !errors.Is(err, ErrMismatch) || replay.Cursor != 0 {
		t.Fatal("invalid text consumed replay", err)
	}
}

func TestResponseBoundaryDoesNotOverwriteOriginalFailure(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   string
		cancel bool
	}{
		{context.Canceled, "canceled", true},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "deadline", false},
		{errors.New("controlled internal error"), "tool_failed", false},
	} {
		ctx, cancel := context.WithCancel(context.Background())
		r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
		text, err := r.Do(ctx, "tool", "read", "q", func(context.Context) (string, error) {
			if tc.cancel {
				cancel()
			}
			return strings.Repeat("x", 4097), tc.err
		})
		cancel()
		if text != "" || !errors.Is(err, tc.err) || r.Archive.Calls[0].ErrorCode != tc.code {
			t.Fatal("original failure overwritten", text, err, r.Archive)
		}
	}
}
