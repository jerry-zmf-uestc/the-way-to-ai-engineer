package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type InternalConfig struct {
	ID, Credential, InternalAddress string
	Scope                           Scope
	TimeoutSeconds, RetryLimit      int
}
type PublicConfig struct {
	DocumentID     string `json:"document_id"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	RetryLimit     int    `json:"retry_limit"`
}

func ProjectConfig(doc Document, config InternalConfig) (Result, error) {
	if !validID(doc.ID) || config.ID != doc.ID || config.Scope != doc.Scope || doc.Scope.Tenant == "" || doc.Scope.User == "" || config.TimeoutSeconds < 1 || config.TimeoutSeconds > 3600 || config.RetryLimit < 0 || config.RetryLimit > 10 {
		return Result{}, ErrResult
	}
	public := PublicConfig{DocumentID: doc.ID, TimeoutSeconds: config.TimeoutSeconds, RetryLimit: config.RetryLimit}
	data, err := json.Marshal(public)
	if err != nil {
		return Result{}, ErrResult
	}
	return Result{ID: doc.ID, Text: string(data)}, nil
}

func projectionDemo() {
	scope := Scope{Tenant: "demo", User: "alice"}
	config := InternalConfig{ID: "guide", Scope: scope, TimeoutSeconds: 30, RetryLimit: 2, Credential: "SYNTHETIC_CANARY_NOT_A_CREDENTIAL", InternalAddress: "https://internal.example.invalid"}
	policy := Policy{Tools: map[string]bool{"read_document": true}, Documents: map[string]bool{"guide": true}, MaxClass: "public", ExpiresAt: time.Now().Add(time.Minute)}
	corpus := map[string]Document{"guide": {ID: "guide", Scope: scope, Class: "public"}}
	gate, err := NewGate("projection-run", scope, policy, corpus, func(_ context.Context, doc Document) (Result, error) { return ProjectConfig(doc, config) })
	if err != nil {
		panic(err)
	}
	evidence, err := gate.Execute(context.Background(), Proposal{Tool: "read_document", Arguments: `{"document_id":"guide"}`})
	if err != nil {
		panic(err)
	}
	fmt.Printf("projection=%s trust=%s\n", evidence.Text, evidence.Trust)
}
