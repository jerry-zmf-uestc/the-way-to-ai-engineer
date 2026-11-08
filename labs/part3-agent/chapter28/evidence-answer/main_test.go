package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func evidence() []Evidence {
	return []Evidence{{ID: "e", Source: "guide", Version: "1", Tenant: "u", Text: "Timeout is 30 seconds.", FactKey: "timeout", FactValue: "30"}}
}

const valid = `{"claims":[{"text":"Timeout is 30 seconds.","evidence_id":"e","version":"1","quote":"Timeout is 30 seconds."}]}`

func scripted(raw string) Model {
	return func(context.Context, string, []Evidence) (string, error) { return raw, nil }
}
func TestAttributionSupportAndNoPartialPublish(t *testing.T) {
	a, err := Run(context.Background(), "u", "q", evidence(), scripted(valid), nil)
	if err != nil || a.Abstained || len(a.Claims) != 1 {
		t.Fatalf("answer=%+v err=%v", a, err)
	}
	for _, raw := range []string{
		`{"claims":[{"text":"invented","evidence_id":"missing","version":"1","quote":"Timeout is 30 seconds."}]}`,
		`{"claims":[{"text":"Timeout is 30 seconds.","evidence_id":"e","version":"old","quote":"Timeout is 30 seconds."}]}`,
		`{"claims":[{"text":"Timeout is 30 seconds.","evidence_id":"e","version":"1","quote":"invented quote"}]}`,
	} {
		a, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), nil)
		if !errors.Is(err, ErrEvidence) || len(a.Claims) != 0 {
			t.Fatalf("invalid attribution accepted: %+v %v", a, err)
		}
	}
	raw := `{"claims":[{"text":"Timeout is 60 seconds.","evidence_id":"e","version":"1","quote":"Timeout is 30 seconds."}]}`
	for _, v := range []Verdict{Unknown, Unsupported} {
		a, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), func(context.Context, Claim, Evidence) (Verdict, error) { return v, nil })
		want := ErrReview
		if v == Unsupported {
			want = ErrUnsupported
		}
		if !errors.Is(err, want) || len(a.Claims) != 0 {
			t.Fatalf("verdict=%s answer=%+v err=%v", v, a, err)
		}
	}
	if _, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), nil); !errors.Is(err, ErrReview) {
		t.Fatal("paraphrase accepted without judgment")
	}
}
func TestEmptyConflictAndScopeBeforeModel(t *testing.T) {
	calls := 0
	model := func(context.Context, string, []Evidence) (string, error) { calls++; return valid, nil }
	a, err := Run(context.Background(), "u", "q", nil, model, nil)
	if err != nil || !a.Abstained || a.Reason != "no_evidence" || calls != 0 {
		t.Fatal("empty evidence generated")
	}
	items := evidence()
	items = append(items, Evidence{ID: "other", Source: "old", Version: "2", Tenant: "u", Text: "Timeout is 60 seconds.", FactKey: "timeout", FactValue: "60"})
	a, err = Run(context.Background(), "u", "q", items, model, nil)
	if err != nil || a.Reason != "conflicting_evidence" || calls != 0 {
		t.Fatal("conflict ignored")
	}
	if _, err := Run(context.Background(), "foreign", "q", evidence(), model, nil); !errors.Is(err, ErrEvidence) || calls != 0 {
		t.Fatal("foreign source sent to model")
	}
}
func TestDraftAndCancellation(t *testing.T) {
	duplicate := `{"claims":[{"text":"Timeout is 60 seconds.","text":"Timeout is 30 seconds.","evidence_id":"e","version":"1","quote":"Timeout is 30 seconds."}]}`
	if _, err := Parse(duplicate); !errors.Is(err, ErrDraft) {
		t.Fatal("duplicate field accepted", err)
	}
	if _, err := Parse(strings.Replace(valid, `}]}`, `},`+strings.TrimSuffix(strings.TrimPrefix(valid, `{"claims":[`), `]}`)+`]}`, 1)); err != nil {
		t.Fatal("keys from separate claims conflict", err)
	}
	for _, raw := range []string{`{}`, `{"claims":[]}`, valid + `{}`, `{"claims":[],"success":true}`, `not json`} {
		if _, err := Parse(raw); !errors.Is(err, ErrDraft) {
			t.Fatalf("bad draft accepted: %s", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := func(context.Context, string, []Evidence) (string, error) { cancel(); return valid, nil }
	if a, err := Run(ctx, "u", "q", evidence(), model, nil); !errors.Is(err, context.Canceled) || len(a.Claims) != 0 {
		t.Fatal("late answer accepted")
	}
}
func TestJudgeFailureKeepsCause(t *testing.T) {
	raw := `{"claims":[{"text":"paraphrase","evidence_id":"e","version":"1","quote":"Timeout is 30 seconds."}]}`
	cause := errors.New("judge unavailable")
	if _, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), func(context.Context, Claim, Evidence) (Verdict, error) { return Unknown, cause }); !errors.Is(err, cause) {
		t.Fatal("judge failure hidden")
	}
}

func TestPartialQuoteCannotBypassSupportJudgment(t *testing.T) {
	for _, item := range []struct{ evidence, quote string }{
		{"本应用不允许跨用户读取。", "允许跨用户读取"},
		{"仅在已获批准时，可以读取文档。", "可以读取文档。"},
	} {
		e := Evidence{ID: "e", Source: "policy", Version: "1", Tenant: "u", Text: item.evidence}
		draft := Draft{Claims: []Claim{{Text: item.quote, Quote: item.quote, EvidenceID: "e", Version: "1"}}}
		registry := map[string]Evidence{"e": e}
		if err := Validate(context.Background(), draft, registry, nil); !errors.Is(err, ErrReview) {
			t.Fatal("partial identity promoted to supported", item, err)
		}
		calls := 0
		judge := func(_ context.Context, claim Claim, full Evidence) (Verdict, error) {
			calls++
			if full.Text != item.evidence || claim.Text != item.quote {
				t.Fatal("context lost before judgment")
			}
			return Unsupported, nil
		}
		if err := Validate(context.Background(), draft, registry, judge); !errors.Is(err, ErrUnsupported) || calls != 1 {
			t.Fatal("support judge bypassed", calls, err)
		}
	}
}

func TestSupportedPartialQuoteRequiresExplicitJudgeAndNoPartialPublish(t *testing.T) {
	items := []Evidence{{ID: "e", Source: "guide", Version: "1", Tenant: "u", Text: "Timeout is 30 seconds. Retry limit is 2."}}
	raw := `{"claims":[{"text":"Timeout is 30 seconds.","quote":"Timeout is 30 seconds.","evidence_id":"e","version":"1"}]}`
	calls := 0
	judge := func(_ context.Context, claim Claim, full Evidence) (Verdict, error) {
		calls++
		if claim.Text == "Timeout is 30 seconds." && full.Text == items[0].Text {
			return Supported, nil
		}
		return Unknown, nil
	}
	a, err := Run(context.Background(), "u", "timeout?", items, scripted(raw), judge)
	if err != nil || len(a.Claims) != 1 || calls != 1 {
		t.Fatal("explicit support not used", a, err, calls)
	}
	items[0].Text = "本应用不允许跨用户读取。"
	full := Claim{Text: items[0].Text, Quote: items[0].Text, EvidenceID: "e", Version: "1"}
	partial := Claim{Text: "允许跨用户读取", Quote: "允许跨用户读取", EvidenceID: "e", Version: "1"}
	if err := Validate(context.Background(), Draft{Claims: []Claim{full, partial}}, map[string]Evidence{"e": items[0]}, nil); !errors.Is(err, ErrReview) {
		t.Fatal("later partial claim bypassed review", err)
	}
	batch, err := json.Marshal(Draft{Claims: []Claim{full, partial}})
	if err != nil {
		t.Fatal(err)
	}
	if answer, err := Run(context.Background(), "u", "policy?", items, scripted(string(batch)), nil); !errors.Is(err, ErrReview) || len(answer.Claims) != 0 {
		t.Fatal("valid earlier claim leaked from failed draft", answer, err)
	}
}
