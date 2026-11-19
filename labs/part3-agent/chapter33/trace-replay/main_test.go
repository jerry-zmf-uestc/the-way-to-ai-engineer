package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func recording(t *testing.T) (Recorder, Archive) {
	t.Helper()
	r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
	_, err := task(context.Background(), func(ctx context.Context, kind, name, input string) (string, error) {
		return r.Do(ctx, kind, name, input, func(context.Context) (string, error) {
			if kind == "model" {
				return "read", nil
			}
			return "", ErrTool
		})
	})
	if !errors.Is(err, ErrTool) {
		t.Fatal(err)
	}
	data, _ := json.Marshal(r.Archive)
	a, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	return r, a
}
func TestReplayReproducesFailureWithoutLiveCallbacks(t *testing.T) {
	_, a := recording(t)
	r := Replay{Archive: a}
	_, err := task(context.Background(), r.Do)
	if !errors.Is(err, ErrTool) || r.Done() != nil || r.Cursor != 2 {
		t.Fatalf("replay=%+v err=%v", r, err)
	}
}
func TestMismatchMissingAndUnconsumedRecords(t *testing.T) {
	_, a := recording(t)
	for _, input := range []string{"changed-question", "question "} {
		r := Replay{Archive: a}
		if _, err := r.Do(context.Background(), "model", "decide", input); !errors.Is(err, ErrMismatch) || r.Cursor != 0 {
			t.Fatal("wrong input matched")
		}
	}
	r := Replay{Archive: a}
	r.Do(context.Background(), "model", "decide", "question")
	if !errors.Is(r.Done(), ErrMismatch) {
		t.Fatal("unconsumed call accepted")
	}
	a.Calls = a.Calls[:1]
	r = Replay{Archive: a}
	_, err := task(context.Background(), r.Do)
	if !errors.Is(err, ErrMismatch) {
		t.Fatal("missing call accepted")
	}
}
func TestPublicEventsExcludePayloadAndPrivateErrors(t *testing.T) {
	r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
	payload := "teaching-private-payload"
	cause := errors.New("internal-only-error")
	r.Do(context.Background(), "model", "respond", "private-input", func(context.Context) (string, error) { return payload, nil })
	r.Do(context.Background(), "tool", "read", "q", func(context.Context) (string, error) { return "", cause })
	data, _ := json.Marshal(r.Events)
	for _, secret := range []string{payload, "private-input", cause.Error()} {
		if strings.Contains(string(data), secret) {
			t.Fatal("payload in public diagnostic")
		}
	}
	if r.Archive.Calls[0].Response != payload {
		t.Fatal("private recording lost payload")
	}
	if err := ValidateEvents(r.Events); err != nil {
		t.Fatal(err)
	}
	r.Events[1].Seq = 9
	if !errors.Is(ValidateEvents(r.Events), ErrTrace) {
		t.Fatal("wrong sequence accepted")
	}
}
func TestCancellationRecordsFailureAndRejectsLateText(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
	text, err := r.Do(ctx, "model", "respond", "q", func(context.Context) (string, error) { cancel(); return "late", nil })
	if text != "" || !errors.Is(err, context.Canceled) || r.Archive.Calls[0].ErrorCode != "canceled" {
		t.Fatal("late output recorded as success")
	}
}
func TestStrictArchive(t *testing.T) {
	for _, data := range []string{`{"Schema":9,"Run":"r","Calls":[]}`, `{"Schema":1,"Run":"r","Calls":[],"extra":1}`, `{"Schema":1,"Run":"r","Calls":[]}{}`} {
		if _, err := Load([]byte(data)); !errors.Is(err, ErrTrace) {
			t.Fatal("invalid archive accepted")
		}
	}
}

func TestProtocolFailureRemainsDistinctInReplay(t *testing.T) {
	r := Recorder{Archive: Archive{Schema: 1, Run: "r"}}
	_, err := r.Do(context.Background(), "model", "respond", "q", func(context.Context) (string, error) { return strings.Repeat("x", 4097), nil })
	if !errors.Is(err, ErrTrace) {
		t.Fatal("oversized response accepted")
	}
	data, _ := json.Marshal(r.Archive)
	a, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}
	replay := Replay{Archive: a}
	if _, err := replay.Do(context.Background(), "model", "respond", "q"); !errors.Is(err, ErrTrace) {
		t.Fatal("protocol failure became model service failure")
	}
}
