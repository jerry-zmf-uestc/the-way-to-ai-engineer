package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStructuredResultThroughActualStdio(t *testing.T) {
	c := processForServer(t, "structured", "structured", map[string]bool{"read_doc": true})
	ctx := context.Background()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Discover(ctx); err != nil {
		t.Fatal(err)
	}
	value, err := c.CallStructured(ctx, "structured::read_doc", map[string]any{"id": "doc-A"}, "4")
	if err != nil || value.TimeoutSeconds != 30 || value.ID != "doc-A" || value.Version != "4" || value.Unit != "seconds" {
		t.Fatal(value, err)
	}
	var projected StructuredDocument
	if json.Unmarshal([]byte(value.Observation()), &projected) != nil || projected != value {
		t.Fatal("projection lost validated fields")
	}
	before := c.seq
	for _, name := range []string{"read_doc", "other::read_doc", "structured::missing"} {
		if _, err := c.CallStructured(ctx, name, map[string]any{"id": "doc-A"}, "4"); !errors.Is(err, ErrDenied) {
			t.Fatal(name, err)
		}
	}
	if c.seq != before {
		t.Fatal("denied call sent RPC")
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if value, err := c.CallStructured(stopped, "structured::read_doc", map[string]any{"id": "doc-A"}, "4"); !errors.Is(err, context.Canceled) || value != (StructuredDocument{}) || c.seq != before {
		t.Fatal("canceled request published or sent", value, err)
	}
	if value, err := c.CallStructured(ctx, "structured::read_doc", map[string]any{"id": "missing"}, "4"); !errors.Is(err, ErrTool) || value != (StructuredDocument{}) {
		t.Fatal("tool error became data", value, err)
	}
	if value, err := c.CallStructured(ctx, "structured::read_doc", map[string]any{"id": "doc-A"}, "3"); !errors.Is(err, ErrProtocol) || value != (StructuredDocument{}) {
		t.Fatal("wrong version accepted", value, err)
	}
}

func TestStructuredCallRequiresDeclaredSchemaBeforeRPC(t *testing.T) {
	c := process(t, map[string]bool{"read_doc": true})
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := c.seq
	if _, err := c.CallStructured(context.Background(), "docs::read_doc", map[string]any{"id": "doc-A"}, "4"); !errors.Is(err, ErrProtocol) || c.seq != before {
		t.Fatal("text tool silently accepted as structured", err, c.seq)
	}
}

func TestStructuredResultRejectsShapeRangeIdentityAndDisagreement(t *testing.T) {
	for _, body := range []string{
		`{"id":"doc-A","timeout_seconds":30,"unit":"seconds","version":"4"}`,
		`{"version":"4","unit":"seconds","timeout_seconds":30,"id":"doc-A"}`,
	} {
		raw, _ := json.Marshal(structuredEnvelope{Content: []Content{{Type: "text", Text: body}}, StructuredContent: json.RawMessage(body)})
		if _, err := parseStructuredResult(raw, "doc-A", "4"); err != nil {
			t.Fatal("valid object order rejected", err)
		}
	}
	base := structuredSuccess()
	for name, body := range map[string]string{
		"case mismatch": `{"ID":"doc-A","timeout_seconds":30,"unit":"seconds","version":"4"}`,
		"missing":       `{"id":"doc-A","timeout_seconds":30,"version":"4"}`,
		"string number": `{"id":"doc-A","timeout_seconds":"30","unit":"seconds","version":"4"}`,
		"zero":          `{"id":"doc-A","timeout_seconds":0,"unit":"seconds","version":"4"}`,
		"over limit":    `{"id":"doc-A","timeout_seconds":3601,"unit":"seconds","version":"4"}`,
		"unit":          `{"id":"doc-A","timeout_seconds":30,"unit":"milliseconds","version":"4"}`,
		"identity":      `{"id":"other","timeout_seconds":30,"unit":"seconds","version":"4"}`,
		"version":       `{"id":"doc-A","timeout_seconds":30,"unit":"seconds","version":"3"}`,
		"duplicate":     `{"id":"doc-A","timeout_seconds":30,"timeout_seconds":60,"unit":"seconds","version":"4"}`,
		"extra":         `{"id":"doc-A","timeout_seconds":30,"unit":"seconds","version":"4","private":"internal"}`,
		"disagreement":  `{"id":"doc-A","timeout_seconds":60,"unit":"seconds","version":"4"}`,
	} {
		t.Run(name, func(t *testing.T) {
			envelope := base
			envelope.StructuredContent = json.RawMessage(body)
			raw, _ := json.Marshal(envelope)
			if value, err := parseStructuredResult(raw, "doc-A", "4"); !errors.Is(err, ErrProtocol) || value != (StructuredDocument{}) {
				t.Fatal("invalid data published", value, err)
			}
		})
	}
	for _, raw := range [][]byte{[]byte("null"), []byte(strings.Repeat("x", 8193)), {0xff}} {
		if _, err := parseStructuredResult(raw, "doc-A", "4"); !errors.Is(err, ErrProtocol) {
			t.Fatal("invalid envelope accepted", err)
		}
	}
	for _, envelope := range []structuredEnvelope{
		{Content: base.Content},
		{Content: []Content{{Type: "image", Text: string(base.StructuredContent)}}, StructuredContent: base.StructuredContent},
		{Content: append(append([]Content(nil), base.Content...), base.Content...), StructuredContent: base.StructuredContent},
	} {
		raw, _ := json.Marshal(envelope)
		if value, err := parseStructuredResult(raw, "doc-A", "4"); !errors.Is(err, ErrProtocol) || value != (StructuredDocument{}) {
			t.Fatal("incomplete or unsupported content accepted", value, err)
		}
	}
}

func TestStructuredSchemaRejectsUnsupportedAndDuplicateDeclarations(t *testing.T) {
	if err := validateStructuredSchema(structuredSchema()); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		strings.Replace(string(structuredSchema()), `"type"`, `"TYPE"`, 1),
		strings.Replace(string(structuredSchema()), `"type":"integer"`, `"TYPE":"integer"`, 1),
		"null", `{}`, strings.Replace(string(structuredSchema()), `"integer"`, `"number"`, 1),
		strings.Replace(string(structuredSchema()), `"id","timeout_seconds"`, `"id","id"`, 1),
		strings.Replace(string(structuredSchema()), `"type":"integer"`, `"type":"integer","minimum":1`, 1),
		strings.Replace(string(structuredSchema()), `"additionalProperties":false`, `"additionalProperties":true`, 1),
	} {
		if err := validateStructuredSchema([]byte(raw)); !errors.Is(err, ErrProtocol) {
			t.Fatal("unsupported schema accepted", raw, err)
		}
	}
}

func TestStructuredEnvelopeFieldCaseIsNotNormalized(t *testing.T) {
	raw, _ := json.Marshal(structuredSuccess())
	for _, field := range []string{"content", "structuredContent"} {
		bad := strings.Replace(string(raw), `"`+field+`"`, `"`+strings.ToUpper(field)+`"`, 1)
		if _, err := parseStructuredResult([]byte(bad), "doc-A", "4"); !errors.Is(err, ErrProtocol) {
			t.Fatal("noncanonical envelope accepted", field, err)
		}
	}
}
