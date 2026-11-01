package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestProjectionWhitelistAndGateAuthorizationRemainSeparate(t *testing.T) {
	config := InternalConfig{ID: "guide", Scope: alice, TimeoutSeconds: 30, RetryLimit: 2, Credential: canary, InternalAddress: canary}
	calls := 0
	gate, _, _ := fixture(t, func(_ context.Context, doc Document) (Result, error) { calls++; return ProjectConfig(doc, config) })
	evidence, err := gate.Execute(context.Background(), Proposal{"read_document", `{"document_id":"guide"}`})
	if err != nil || evidence.Trust != "untrusted" || calls != 1 {
		t.Fatal(evidence, err, calls)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(evidence.Text), &fields); err != nil || len(fields) != 3 || string(fields["document_id"]) != `"guide"` || string(fields["timeout_seconds"]) != "30" || string(fields["retry_limit"]) != "2" {
		t.Fatal("public contract changed", fields, err)
	}
	for _, id := range []string{"other", "secret"} {
		if _, err := gate.Execute(context.Background(), Proposal{"read_document", `{"document_id":"` + id + `"}`}); !errors.Is(err, ErrDenied) || calls != 1 {
			t.Fatal("projection bypassed authorization", id, err, calls)
		}
	}
	audit, err := json.Marshal(gate.Audit())
	if err != nil || strings.Contains(evidence.Text, canary) || strings.Contains(string(audit), canary) || strings.Contains(evidence.Text, "Credential") || strings.Contains(evidence.Text, "InternalAddress") {
		t.Fatal("private fields released", evidence, string(audit), err)
	}
}

func TestProjectionChecksRecordIdentityAndNumericBounds(t *testing.T) {
	doc := Document{ID: "guide", Scope: alice}
	base := InternalConfig{ID: doc.ID, Scope: alice, TimeoutSeconds: 1, RetryLimit: 0}
	for _, edit := range []func(*InternalConfig){
		func(c *InternalConfig) { c.ID = "other" }, func(c *InternalConfig) { c.Scope.User = "bob" },
		func(c *InternalConfig) { c.TimeoutSeconds = 0 }, func(c *InternalConfig) { c.TimeoutSeconds = 3601 },
		func(c *InternalConfig) { c.RetryLimit = -1 }, func(c *InternalConfig) { c.RetryLimit = 11 },
	} {
		c := base
		edit(&c)
		if result, err := ProjectConfig(doc, c); !errors.Is(err, ErrResult) || result != (Result{}) {
			t.Fatal("invalid record released", result, err)
		}
	}
	for _, c := range []InternalConfig{base, {ID: doc.ID, Scope: alice, TimeoutSeconds: 3600, RetryLimit: 10}} {
		if _, err := ProjectConfig(doc, c); err != nil {
			t.Fatal("valid boundary rejected", err)
		}
	}
}
