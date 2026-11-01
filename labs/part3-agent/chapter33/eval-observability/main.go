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
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrRecord         = errors.New("eval-observability: invalid archive or replay")
	ErrTool           = errors.New("eval-observability: tool failed")
	ErrReplay         = errors.New("eval-observability: replay mismatch")
	ErrAnswerProtocol = errors.New("eval-observability: answer protocol failed")
)

type Input struct{ Doc, Tenant string }
type Observation struct{ Doc, Version, Text, Missing string }
type Answer struct {
	Text, Source, Version string
	Abstained             bool
}
type Case struct {
	ID, Split string
	Input     Input
	Expected  Answer
	Critical  bool
}
type Call struct {
	Seq                            int
	Kind, InputHash, Output, Error string
	Metrics                        *CallMetrics `json:",omitempty"`
}
type Run struct {
	CaseID, RunID string
	ToolLimit     int `json:",omitempty"`
	FinalLimit    int `json:",omitempty"`
	Calls         []Call
	Answer        Answer
	Error         string
	ElapsedNS     int64
}
type Archive struct {
	Schema                  int
	Dataset, Config, Corpus string
	Runs                    []Run
}
type Row struct {
	CaseID, RunID, Split, Error string
	Passed, Critical            bool
	Steps, Attempts             int
	ReplayVerified              bool
	ElapsedNS                   int64
}
type Report struct {
	Dataset, Config, Scorer                        string
	Total, Passed, CriticalFailed, ExecutionFailed int
	UsageKnown, CostKnown, Gate                    bool
	Rows                                           []Row
	Regressions                                    []string
	CallUsage                                      *UsageSummary `json:",omitempty"`
}

func cases() []Case {
	return []Case{
		{"timeout", "development", Input{"timeout", "demo"}, Answer{"30 seconds", "timeout", "v1", false}, false},
		{"retry", "regression", Input{"retry", "demo"}, Answer{"2 retries", "retry", "v1", false}, false},
		{"absent", "adversarial", Input{"absent", "demo"}, Answer{Abstained: true}, true},
		{"foreign", "production-like", Input{"foreign", "demo"}, Answer{Abstained: true}, true},
	}
}

// The model substitute sees task input or observation, never Case.Expected.
func decision(ctx context.Context, config, kind, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	switch kind {
	case "model-select":
		return input, nil
	case "tool-read":
		var request Input
		if err := decodeJSON([]byte(input), &request, 4096); err != nil {
			return "", ErrRecord
		}
		observation := Observation{Doc: request.Doc, Missing: "absent"}
		if request.Tenant != "demo" || request.Doc == "foreign" {
			observation.Missing = "denied"
		} else if request.Doc == "timeout" {
			observation = Observation{Doc: "timeout", Version: "v1", Text: "30 seconds"}
		} else if request.Doc == "retry" {
			if config == "tool-failure" {
				return "", ErrTool
			}
			observation = Observation{Doc: "retry", Version: "v1", Text: "2 retries"}
		}
		return encoded(observation), nil
	case "model-final":
		var observation Observation
		if err := decodeJSON([]byte(input), &observation, 4096); err != nil {
			return "", ErrRecord
		}
		answer := Answer{Text: observation.Text, Source: observation.Doc, Version: observation.Version}
		if observation.Missing != "" {
			answer = Answer{Abstained: true}
			if config == "regressed" && observation.Missing == "absent" {
				answer = Answer{Text: "0 seconds"}
			}
		}
		return encoded(answer), nil
	default:
		return "", ErrRecord
	}
}
func encoded(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type Driver func(context.Context, string, string) (string, error)

func execute(ctx context.Context, input Input, driver Driver) (Answer, error) {
	return executeBounded(ctx, input, driver, 1)
}

func executeBounded(ctx context.Context, input Input, driver Driver, toolLimit int) (Answer, error) {
	return executeLimited(ctx, input, driver, toolLimit, 1)
}

func executeLimited(ctx context.Context, input Input, driver Driver, toolLimit, finalLimit int) (Answer, error) {
	if toolLimit < 1 || toolLimit > 2 || finalLimit < 1 || finalLimit > 2 {
		return Answer{}, ErrRecord
	}
	if ctx.Err() != nil {
		return Answer{}, ctx.Err()
	}
	selected, err := driver(ctx, "model-select", encoded(input))
	if err != nil {
		return Answer{}, err
	}
	var observation string
	for attempt := 0; attempt < toolLimit; attempt++ {
		if err := ctx.Err(); err != nil {
			return Answer{}, err
		}
		observation, err = driver(ctx, "tool-read", selected)
		if stop := ctx.Err(); stop != nil {
			return Answer{}, stop
		}
		if !errors.Is(err, ErrTool) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
	}
	if err != nil {
		return Answer{}, err
	}
	var raw string
	for attempt := 0; attempt < finalLimit; attempt++ {
		if ctx.Err() != nil {
			return Answer{}, ctx.Err()
		}
		raw, err = driver(ctx, "model-final", observation)
		if ctx.Err() != nil {
			return Answer{}, ctx.Err()
		}
		if !errors.Is(err, ErrAnswerProtocol) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
	}
	if err != nil {
		return Answer{}, err
	}
	var answer Answer
	if decodeJSON([]byte(raw), &answer, 4096) != nil {
		return Answer{}, ErrRecord
	}
	if err := ctx.Err(); err != nil {
		return Answer{}, err
	}
	return answer, nil
}

func record(ctx context.Context, config string) (Archive, error) {
	if !validConfig(config) {
		return Archive{}, ErrRecord
	}
	archive := Archive{Schema: 2, Dataset: "contract-v1", Config: config, Corpus: "readonly-v1"}
	toolLimit := 1
	if config == "tool-recovered" || config == "tool-exhausted" {
		archive.Schema, toolLimit = 3, 2
	}
	for _, c := range cases() {
		run := Run{CaseID: c.ID, RunID: config + "/" + c.ID}
		if archive.Schema == 3 {
			run.ToolLimit = toolLimit
		}
		toolAttempts := 0
		start := time.Now()
		answer, err := executeBounded(ctx, c.Input, func(ctx context.Context, kind, input string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			callConfig := config
			if kind == "tool-read" {
				toolAttempts++
				if config == "tool-exhausted" || (config == "tool-recovered" && toolAttempts == 1) {
					callConfig = "tool-failure"
				}
			}
			output, err := decision(ctx, callConfig, kind, input)
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			if err != nil {
				output = ""
			}
			run.Calls = append(run.Calls, Call{Seq: len(run.Calls) + 1, Kind: kind, InputHash: hash(input), Output: output, Error: errorCode(err)})
			return output, err
		}, toolLimit)
		run.ElapsedNS = time.Since(start).Nanoseconds()
		run.Answer = answer
		if err != nil {
			run.Error = errorCode(err)
		}
		archive.Runs = append(archive.Runs, run)
	}
	return archive, nil
}

// Replay has no live fallback; every recorded request and response must be consumed.
func replay(ctx context.Context, c Case, run Run) (Answer, error) {
	if err := ctx.Err(); err != nil {
		return Answer{}, err
	}
	cursor := 0
	toolLimit := run.ToolLimit
	if toolLimit == 0 {
		toolLimit = 1
	}
	finalLimit := run.FinalLimit
	if finalLimit == 0 {
		finalLimit = 1
	}
	answer, err := executeLimited(ctx, c.Input, func(ctx context.Context, kind, input string) (string, error) {
		if cursor >= len(run.Calls) {
			if run.Error == "canceled" || run.Error == "deadline" {
				return "", errorFor(run.Error)
			}
			return "", ErrReplay
		}
		call := run.Calls[cursor]
		if call.Seq != cursor+1 || call.Kind != kind || call.InputHash != hash(input) || len(call.Output) > 4096 {
			return "", ErrReplay
		}
		cursor++
		return call.Output, errorFor(call.Error)
	}, toolLimit, finalLimit)
	if cursor != len(run.Calls) {
		return Answer{}, ErrReplay
	}
	if err == nil && (run.Error == "canceled" || run.Error == "deadline") {
		// Reproduce a recorded terminal stop, not its original timing.
		return Answer{}, errorFor(run.Error)
	}
	return answer, err
}
func errorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, ErrTool):
		return "tool_failed"
	case errors.Is(err, ErrAnswerProtocol):
		return "answer_protocol"
	default:
		return "protocol_failed"
	}
}
func errorFor(code string) error {
	switch code {
	case "":
		return nil
	case "canceled":
		return context.Canceled
	case "deadline":
		return context.DeadlineExceeded
	case "tool_failed":
		return ErrTool
	case "answer_protocol":
		return ErrAnswerProtocol
	default:
		return ErrRecord
	}
}
func validCode(code string) bool {
	return code == "" || code == "canceled" || code == "deadline" || code == "tool_failed" || code == "protocol_failed" || code == "answer_protocol"
}

func validConfig(config string) bool {
	return config == "baseline" || config == "regressed" || config == "tool-failure" || config == "tool-recovered" || config == "tool-exhausted"
}

func evaluate(ctx context.Context, archive Archive) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	retrying := archive.Config == "tool-recovered" || archive.Config == "tool-exhausted"
	repairing := archive.Config == "protocol-repair"
	if archive.Schema < 1 || archive.Schema > 5 || archive.Dataset != "contract-v1" || archive.Corpus != "readonly-v1" || (!validConfig(archive.Config) && !repairing) || (archive.Schema == 5) != repairing || (archive.Schema < 4 && (archive.Schema == 3) != retrying) || (archive.Schema == 1 && archive.Config == "tool-failure") || len(archive.Runs) != len(cases()) {
		return Report{}, ErrRecord
	}
	report := Report{Dataset: archive.Dataset, Config: archive.Config, Scorer: "exact-contract-v2", Total: len(cases())}
	if archive.Schema >= 4 {
		report.CallUsage = &UsageSummary{}
	}
	seen := map[string]bool{}
	for n, c := range cases() {
		run := archive.Runs[n]
		maxCalls := 3
		if retrying {
			maxCalls = 4
		}
		if repairing {
			maxCalls = 4
		}
		if repairing && run.FinalLimit != 2 || !repairing && run.FinalLimit != 0 {
			return Report{}, ErrRecord
		}
		if (retrying && run.ToolLimit != 2) || (!retrying && run.ToolLimit != 0) || run.CaseID != c.ID || run.RunID != archive.Config+"/"+c.ID || seen[run.RunID] || run.ElapsedNS < 0 || len(run.Calls) > maxCalls || (!validCode(run.Error) && !(archive.Schema == 1 && run.Error == "execution_failed")) || (archive.Schema == 1 && run.Error != "" && run.Error != "execution_failed") {
			return Report{}, ErrRecord
		}
		seen[run.RunID] = true
		for i, call := range run.Calls {
			digest, err := hex.DecodeString(call.InputHash)
			validKind := call.Kind == "model-select" || call.Kind == "tool-read" || call.Kind == "model-final"
			if call.Seq != i+1 || !validKind || (!retrying && !repairing && call.Kind != []string{"model-select", "tool-read", "model-final"}[i]) || err != nil || len(digest) != 32 || len(call.Output) > 4096 || !utf8.ValidString(call.Output) || !validCode(call.Error) || (call.Error == "tool_failed" && call.Kind != "tool-read") || (call.Error == "answer_protocol" && (!repairing || call.Kind != "model-final")) || (call.Error != "" && call.Output != "") || (archive.Schema == 1 && call.Error != "") {
				return Report{}, ErrRecord
			}
			if archive.Schema >= 4 {
				if !validCallMetrics(call.Kind, call.Metrics) {
					return Report{}, ErrRecord
				}
				report.CallUsage.add(*call.Metrics)
			} else if call.Metrics != nil {
				return Report{}, ErrRecord
			}
		}
		if run.Error != "" {
			report.ExecutionFailed++
		}
		passed, verified := false, true
		if archive.Schema == 1 && run.Error != "" {
			verified = false // Legacy failure summaries did not include failed calls.
			if run.Answer != (Answer{}) {
				return Report{}, ErrRecord
			}
		} else {
			answer, err := replay(ctx, c, run)
			if stop := ctx.Err(); stop != nil {
				return Report{}, stop
			}
			if errors.Is(err, ErrReplay) || errorCode(err) != run.Error || answer != run.Answer {
				return Report{}, ErrRecord
			}
			passed = err == nil && answer == c.Expected
		}
		steps := 0
		for _, call := range run.Calls {
			if call.Error == "" {
				steps++
			}
		}

		if passed {
			report.Passed++
		} else if c.Critical {
			report.CriticalFailed++
		}
		report.Rows = append(report.Rows, Row{c.ID, run.RunID, c.Split, run.Error, passed, c.Critical, steps, len(run.Calls), verified, run.ElapsedNS})
	}
	// A 75% average cannot compensate for a required refusal failure.
	report.Gate = report.Passed*4 >= report.Total*3 && report.CriticalFailed == 0 && report.ExecutionFailed == 0
	if report.CallUsage != nil {
		report.UsageKnown = report.CallUsage.ModelCalls > 0 && report.CallUsage.UnknownModelUsage == 0
	}
	return report, nil
}

func uniqueJSON(dec *json.Decoder, depth int) error {
	if depth > 12 {
		return ErrRecord
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); ok {
		seen := map[string]bool{}
		for dec.More() {
			if delimiter == '{' {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[strings.ToLower(name)] {
					return ErrRecord
				}
				seen[strings.ToLower(name)] = true
			}
			if err := uniqueJSON(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
	}
	return err
}
func decodeJSON(data []byte, target any, limit int) error {
	if len(data) > limit || !utf8.Valid(data) || uniqueJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return ErrRecord
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return ErrRecord
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrRecord
	}
	return nil
}
func load(reader io.Reader) (Archive, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (128<<10)+1))
	if err != nil {
		return Archive{}, ErrRecord
	}
	var archive Archive
	if decodeJSON(data, &archive, 128<<10) != nil {
		return Archive{}, ErrRecord
	}
	return archive, nil
}

func compare(base, next Report) ([]string, error) {
	if base.Dataset != next.Dataset || base.Scorer != next.Scorer || len(base.Rows) != len(next.Rows) || len(base.Rows) == 0 {
		return nil, ErrRecord
	}
	regressions := []string{}
	for n, before := range base.Rows {
		after := next.Rows[n]
		if before.CaseID != after.CaseID || before.Split != after.Split || before.Critical != after.Critical {
			return nil, ErrRecord
		}
		if before.Passed && !after.Passed {
			regressions = append(regressions, before.CaseID)
		}
	}
	return regressions, nil
}

func readArchive(path string) (Archive, error) {
	file, err := os.Open(path)
	if err != nil {
		return Archive{}, err
	}
	defer file.Close()
	return load(file)
}

func main() {
	config := flag.String("config", "baseline", "baseline, regressed, tool-failure, tool-recovered, or tool-exhausted; fixed offline substitute")
	archiveOnly := flag.Bool("archive", false, "write original synthetic calls to stdout")
	replayFile := flag.String("replay", "", "read a local archive; never execute live calls")
	compareFile := flag.String("compare", "", "compare with a local baseline archive by replay only")
	flag.Parse()
	var archive Archive
	var err error
	if *replayFile != "" {
		archive, err = readArchive(*replayFile)
	} else {
		archive, err = record(context.Background(), *config)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, ErrRecord)
		os.Exit(2)
	}
	report, err := evaluate(context.Background(), archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, ErrRecord)
		os.Exit(2)
	}
	if *archiveOnly {
		fmt.Println(encoded(archive))
		return
	}
	if *compareFile != "" {
		baseline, readErr := readArchive(*compareFile)
		var before Report
		if readErr == nil {
			before, readErr = evaluate(context.Background(), baseline)
		}
		if readErr == nil {
			report.Regressions, readErr = compare(before, report)
		}
		if readErr != nil {
			fmt.Fprintln(os.Stderr, ErrRecord)
			os.Exit(2)
		}
		report.Gate = report.Gate && len(report.Regressions) == 0
	}
	fmt.Println(encoded(report))
	if !report.Gate {
		os.Exit(1)
	}
}
