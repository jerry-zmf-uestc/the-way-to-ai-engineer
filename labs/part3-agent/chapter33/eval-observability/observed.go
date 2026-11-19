package main

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type CallMetrics struct {
	Origin, ModelRevision, UsageSource string
	UsageKnown                         bool
	InputTokens, OutputTokens          int64
}
type ObservedReply struct {
	Output  string
	Metrics CallMetrics
}
type ObservedDriver func(context.Context, string, string) (ObservedReply, error)
type UsageSummary struct {
	ApplicationCalls, ToolCalls, FixedCalls, ModelCalls int
	UnknownModelUsage                                   int
	KnownInputTokens, KnownOutputTokens                 int64
	SyntheticCalls, UnknownSyntheticUsage               int
	SyntheticInputTokens, SyntheticOutputTokens         int64
}

func validCallMetrics(kind string, m *CallMetrics) bool {
	if m == nil || m.InputTokens < 0 || m.OutputTokens < 0 || m.InputTokens > 1e9 || m.OutputTokens > 1e9 {
		return false
	}
	if m.Origin == "model" || m.Origin == "synthetic-model" {
		if kind == "tool-read" || strings.TrimSpace(m.ModelRevision) == "" || len(m.ModelRevision) > 128 || !utf8.ValidString(m.ModelRevision) {
			return false
		}
		if m.UsageKnown {
			if m.Origin == "synthetic-model" {
				return m.UsageSource == "synthetic"
			}
			return m.UsageSource == "local-tokenizer" || m.UsageSource == "provider-reported"
		}
		return m.UsageSource == "" && m.InputTokens == 0 && m.OutputTokens == 0
	}
	if kind == "tool-read" && m.Origin != "tool" || kind != "tool-read" && m.Origin == "tool" {
		return false
	}
	return (m.Origin == "application" || m.Origin == "fixed-substitute" || m.Origin == "tool") && m.ModelRevision == "" && !m.UsageKnown && m.UsageSource == "" && m.InputTokens == 0 && m.OutputTokens == 0
}

func (s *UsageSummary) add(m CallMetrics) {
	switch m.Origin {
	case "application":
		s.ApplicationCalls++
	case "tool":
		s.ToolCalls++
	case "fixed-substitute":
		s.FixedCalls++
	case "model":
		s.ModelCalls++
		if m.UsageKnown {
			s.KnownInputTokens += m.InputTokens
			s.KnownOutputTokens += m.OutputTokens
		} else {
			s.UnknownModelUsage++
		}
	case "synthetic-model":
		s.SyntheticCalls++
		if m.UsageKnown {
			s.SyntheticInputTokens += m.InputTokens
			s.SyntheticOutputTokens += m.OutputTokens
		} else {
			s.UnknownSyntheticUsage++
		}
	}
}

func recordObserved(ctx context.Context, config string, driver ObservedDriver) (Archive, error) {
	if (!validConfig(config) && config != "protocol-repair") || driver == nil {
		return Archive{}, ErrRecord
	}
	archive := Archive{Schema: 4, Dataset: "contract-v1", Config: config, Corpus: "readonly-v1"}
	finalLimit := 1
	if config == "protocol-repair" {
		archive.Schema, finalLimit = 5, 2
	}
	limit := 1
	if config == "tool-recovered" || config == "tool-exhausted" {
		limit = 2
	}
	for _, c := range cases() {
		run := Run{CaseID: c.ID, RunID: config + "/" + c.ID}
		if finalLimit == 2 {
			run.FinalLimit = 2
		}
		if limit == 2 {
			run.ToolLimit = limit
		}
		started := time.Now()
		invalidCapture := false
		answer, err := executeLimited(ctx, c.Input, func(ctx context.Context, kind, input string) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			reply, err := driver(ctx, kind, input)
			if !validCallMetrics(kind, &reply.Metrics) {
				invalidCapture = true
				return "", ErrRecord
			}
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			if err != nil {
				reply.Output = ""
			}
			if len(reply.Output) > 4096 || !utf8.ValidString(reply.Output) {
				invalidCapture = true
				return "", ErrRecord
			}
			run.Calls = append(run.Calls, Call{Seq: len(run.Calls) + 1, Kind: kind, InputHash: hash(input), Output: reply.Output, Error: errorCode(err), Metrics: &reply.Metrics})
			return reply.Output, err
		}, limit, finalLimit)
		if invalidCapture {
			return Archive{}, ErrRecord
		}
		run.ElapsedNS, run.Answer, run.Error = time.Since(started).Nanoseconds(), answer, errorCode(err)
		archive.Runs = append(archive.Runs, run)
	}
	return archive, nil
}
