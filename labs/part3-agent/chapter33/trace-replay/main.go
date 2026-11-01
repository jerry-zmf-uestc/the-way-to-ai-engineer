package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"unicode/utf8"
)

var (
	ErrTrace    = errors.New("trace: invalid recording")
	ErrMismatch = errors.New("trace: replay mismatch")
	ErrTool     = errors.New("trace: recorded tool failure")
	ErrModel    = errors.New("trace: recorded model failure")
)

type Event struct {
	Seq                                                                  int
	Run, Step, Call, Kind, Name, Phase, InputHash, OutputHash, ErrorCode string
}
type RecordedCall struct{ Kind, Name, InputHash, Response, ErrorCode string }
type Archive struct {
	Schema int
	Run    string
	Calls  []RecordedCall
}
type Recorder struct {
	Archive Archive
	Events  []Event
}
type Driver func(context.Context, string, string, string) (string, error)

func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
func classify(kind string, err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, ErrTrace):
		return "protocol_failed"
	case kind == "model":
		return "model_failed"
	default:
		return "tool_failed"
	}
}
func validCode(code string) bool {
	switch code {
	case "", "canceled", "deadline", "protocol_failed", "model_failed", "tool_failed":
		return true
	default:
		return false
	}
}

func validKindCode(kind, code string) bool {
	if (kind != "model" && kind != "tool") || !validCode(code) {
		return false
	}
	return !(kind == "model" && code == "tool_failed") && !(kind == "tool" && code == "model_failed")
}
func errorFor(code string) error {
	switch code {
	case "":
		return nil
	case "canceled":
		return context.Canceled
	case "deadline":
		return context.DeadlineExceeded
	case "protocol_failed":
		return ErrTrace
	case "model_failed":
		return ErrModel
	case "tool_failed":
		return ErrTool
	default:
		return ErrTrace
	}
}
func (r *Recorder) Do(ctx context.Context, kind, name, input string, live func(context.Context) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.Archive.Schema != 1 || r.Archive.Run == "" || !utf8.ValidString(r.Archive.Run) || (kind != "model" && kind != "tool") || name == "" || len(name) > 128 || !utf8.ValidString(name) || len(input) > 4096 || !utf8.ValidString(input) || len(r.Archive.Calls) >= 16 || live == nil {
		return "", ErrTrace
	}
	n := len(r.Archive.Calls) + 1
	call := fmt.Sprintf("call-%d", n)
	step := fmt.Sprintf("step-%d", n)
	event := Event{Seq: len(r.Events) + 1, Run: r.Archive.Run, Step: step, Call: call, Kind: kind, Name: name, Phase: "started", InputHash: digest(input)}
	r.Events = append(r.Events, event)
	response, err := live(ctx)
	if stop := ctx.Err(); stop != nil {
		err = stop
	}
	if len(response) > 4096 || !utf8.ValidString(response) {
		response = ""
		if err == nil {
			err = ErrTrace
		}
	}
	code := classify(kind, err)
	if err != nil {
		response = ""
	}
	r.Archive.Calls = append(r.Archive.Calls, RecordedCall{Kind: kind, Name: name, InputHash: event.InputHash, Response: response, ErrorCode: code})
	event.Seq = len(r.Events) + 1
	event.Phase = "finished"
	event.OutputHash = digest(response)
	event.ErrorCode = code
	r.Events = append(r.Events, event)
	return response, err
}
func Load(data []byte) (Archive, error) {
	if len(data) > 80<<10 || !utf8.Valid(data) || uniqueArchiveJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return Archive{}, ErrTrace
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var a Archive
	if err := dec.Decode(&a); err != nil {
		return Archive{}, fmt.Errorf("%w: %w", ErrTrace, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Archive{}, ErrTrace
	}
	if a.Schema != 1 || a.Run == "" || len(a.Calls) > 16 {
		return Archive{}, ErrTrace
	}
	for _, c := range a.Calls {
		decoded, err := hex.DecodeString(c.InputHash)
		if err != nil || len(decoded) != 32 || !validKindCode(c.Kind, c.ErrorCode) || c.Name == "" || len(c.Name) > 128 || len(c.Response) > 4096 || c.ErrorCode != "" && c.Response != "" {
			return Archive{}, ErrTrace
		}
	}
	return a, nil
}

type Replay struct {
	Archive Archive
	Cursor  int
}

// Do没有live参数，按请求身份与顺序返回录制结果，不重新执行工具。
func (r *Replay) Do(ctx context.Context, kind, name, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r.Cursor < 0 || r.Cursor >= len(r.Archive.Calls) || len(input) > 4096 || !utf8.ValidString(input) || !utf8.ValidString(name) {
		return "", ErrMismatch
	}
	record := r.Archive.Calls[r.Cursor]
	if record.Kind != kind || record.Name != name || record.InputHash != digest(input) {
		return "", ErrMismatch
	}
	r.Cursor++
	return record.Response, errorFor(record.ErrorCode)
}
func (r *Replay) Done() error {
	if r.Cursor != len(r.Archive.Calls) {
		return ErrMismatch
	}
	return nil
}
func ValidateEvents(events []Event) error {
	if len(events) == 0 || len(events) > 32 || len(events)%2 != 0 {
		return ErrTrace
	}
	seen := map[string]bool{}
	for n := 0; n < len(events); n += 2 {
		start, end := events[n], events[n+1]
		key := start.Run + "/" + start.Call
		input, ie := hex.DecodeString(start.InputHash)
		output, oe := hex.DecodeString(end.OutputHash)
		if ie != nil || oe != nil || len(input) != 32 || len(output) != 32 || !validKindCode(start.Kind, end.ErrorCode) || start.ErrorCode != "" || start.OutputHash != "" {
			return ErrTrace
		}
		if start.Step == "" || start.Call == "" || seen[key] || (start.Kind != "model" && start.Kind != "tool") || start.Name == "" {
			return ErrTrace
		}
		seen[key] = true
		if start.Seq != n+1 || end.Seq != n+2 || start.Run == "" || start.Run != end.Run || start.Step != end.Step || start.Call != end.Call || start.Kind != end.Kind || start.Name != end.Name || start.InputHash != end.InputHash || start.Phase != "started" || end.Phase != "finished" {
			return ErrTrace
		}
	}
	return nil
}
func task(ctx context.Context, driver Driver) (string, error) {
	decision, err := driver(ctx, "model", "decide", "question")
	if err != nil {
		return "", fmt.Errorf("task: model: %w", err)
	}
	if decision != "read" {
		return "", ErrMismatch
	}
	text, err := driver(ctx, "tool", "read", "doc-v1")
	if err != nil {
		return "", fmt.Errorf("task: tool: %w", err)
	}
	return text, nil
}
func main() {
	useSDK := flag.Bool("otel", false, "create real SDK spans with in-memory export and context propagation")
	flag.Parse()
	if *useSDK {
		otelDemo()
		return
	}
	r := Recorder{Archive: Archive{Schema: 1, Run: "demo-run"}}
	_, liveErr := task(context.Background(), func(ctx context.Context, kind, name, input string) (string, error) {
		return r.Do(ctx, kind, name, input, func(context.Context) (string, error) {
			if kind == "model" {
				return "read", nil
			}
			return "", ErrTool
		})
	})
	data, err := json.Marshal(r.Archive)
	if err != nil {
		panic(err)
	}
	archive, err := Load(data)
	if err != nil {
		panic(err)
	}
	replay := Replay{Archive: archive}
	_, replayErr := task(context.Background(), replay.Do)
	if err := replay.Done(); err != nil {
		panic(err)
	}
	if err := ValidateEvents(r.Events); err != nil {
		panic(err)
	}
	fmt.Printf("calls=%d events=%d live_tool_failure=%t replay_tool_failure=%t consumed=%d\n", len(archive.Calls), len(r.Events), errors.Is(liveErr, ErrTool), errors.Is(replayErr, ErrTool), replay.Cursor)
}
