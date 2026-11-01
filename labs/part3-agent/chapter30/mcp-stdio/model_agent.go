package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var ErrModel = errors.New("mcp agent: model boundary failed")

type ModelView struct {
	Tools   []string
	History []LoopMessage
	CallID  string `json:",omitempty"`
}
type GeneratedReply struct {
	Raw                       string
	InputTokens, OutputTokens int
	AtOutputLimit             bool
}
type ModelRecord struct {
	View ModelView
	GeneratedReply
	Stage       string
	BoundCallID string `json:",omitempty"`
}
type ModelRun struct {
	Complete, CoveragePass                                 bool
	Dataset, Prompt, ModelRevision, Error, SemanticQuality string
	Run                                                    LoopReport
	Calls                                                  []ModelRecord
}

type cappedOutput struct{ bytes.Buffer }

func (b *cappedOutput) Write(p []byte) (int, error) {
	if len(p) > (32<<10)-b.Len() {
		return 0, ErrModel
	}
	return b.Buffer.Write(p)
}

func parseGenerated(g GeneratedReply) (LoopReply, error) {
	if g.InputTokens <= 0 || g.InputTokens > 1024 || g.OutputTokens <= 0 || g.OutputTokens > 192 || g.AtOutputLimit || g.OutputTokens == 192 {
		return LoopReply{}, ErrModel
	}
	var reply LoopReply
	if decodeContext([]byte(g.Raw), &reply) != nil {
		return LoopReply{}, ErrModel
	}
	return reply, nil
}

func bindGenerated(g GeneratedReply, callID string) (LoopReply, error) {
	reply, err := parseGenerated(g)
	if err != nil {
		return LoopReply{}, err
	}
	if reply.Final != "" {
		if reply.Call != (LoopCall{}) {
			return LoopReply{}, ErrModel
		}
		return reply, nil
	}
	if callID == "" || len(callID) > 128 || reply.Call.ID != "" || reply.Call.Tool == "" || reply.Call.DocumentID == "" {
		return LoopReply{}, ErrModel
	}
	reply.Call.ID = callID
	return reply, nil
}

func modelDecision(names []string, records *[]ModelRecord) Decision {
	names = append([]string(nil), names...)
	contract := os.Getenv("MCP_MODEL_CONTRACT")
	return func(ctx context.Context, history []LoopMessage) (LoopReply, error) {
		view := ModelView{Tools: append([]string(nil), names...), History: append([]LoopMessage(nil), history...)}
		if contract == "v2" {
			view.CallID = fmt.Sprintf("model-%d", len(*records)+1)
		}
		record := ModelRecord{View: view, Stage: "model_process_failed"}
		defer func() { *records = append(*records, record) }()
		if os.Getenv("MCP_MODEL_E2E") != "1" {
			return LoopReply{}, ErrModel
		}
		if contract != "" && contract != "v1" && contract != "v2" {
			return LoopReply{}, ErrModel
		}
		python, script, assets := os.Getenv("MCP_MODEL_PYTHON"), os.Getenv("MCP_MODEL_SCRIPT"), os.Getenv("MCP_MODEL_ASSETS")
		if python == "" || script == "" || assets == "" {
			return LoopReply{}, ErrModel
		}
		input, err := json.Marshal(view)
		if err != nil || len(input) > 16<<10 {
			return LoopReply{}, ErrModel
		}
		// ponytail: at most four CPU calls; reload assets per call until measured startup cost warrants a persistent worker.
		cmd := exec.CommandContext(ctx, python, script, "--assets", assets)
		cmd.Stdin = bytes.NewReader(input)
		var output cappedOutput
		cmd.Stdout = &output
		cmd.Stderr = os.Stderr
		if cmd.Run() != nil {
			return LoopReply{}, ErrModel
		}
		if decodeContext(output.Bytes(), &record.GeneratedReply) != nil {
			return LoopReply{}, ErrModel
		}
		record.Stage = "invalid_proposal"
		if record.AtOutputLimit || record.OutputTokens == 192 {
			record.Stage = "output_limit"
		}
		reply, err := parseGenerated(record.GeneratedReply)
		if contract == "v2" {
			reply, err = bindGenerated(record.GeneratedReply, view.CallID)
			if err == nil && reply.Call.ID != "" {
				record.BoundCallID = reply.Call.ID
			}
		}
		if err == nil {
			record.Stage = "decoded"
		}
		return reply, err
	}
}

func routingCoverage(r LoopReport, err error) bool {
	if err != nil || !strings.Contains(r.Final, "30") || !strings.Contains(r.Final, "45") {
		return false
	}
	seen := map[string]bool{}
	for _, m := range r.History {
		if m.Role == "tool" && !m.Failed {
			seen[m.Tool] = true
		}
	}
	return seen["primary::read_doc"] && seen["archive::read_doc"]
}
