package main

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

type StructuredDocument struct {
	ID             string `json:"id"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Unit           string `json:"unit"`
	Version        string `json:"version"`
}

type structuredEnvelope struct {
	Content           []Content       `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

func structuredSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"timeout_seconds":{"type":"integer"},"unit":{"type":"string"},"version":{"type":"string"}},"required":["id","timeout_seconds","unit","version"],"additionalProperties":false}`)
}

func validateStructuredSchema(raw []byte) error {
	var fields map[string]json.RawMessage
	if decodeContext(raw, &fields) != nil || len(fields) != 4 {
		return ErrProtocol
	}
	for _, name := range []string{"type", "properties", "required", "additionalProperties"} {
		if _, ok := fields[name]; !ok {
			return ErrProtocol
		}
	}
	var properties map[string]json.RawMessage
	if decodeContext(fields["properties"], &properties) != nil {
		return ErrProtocol
	}
	for _, rawProperty := range properties {
		var property map[string]json.RawMessage
		if decodeContext(rawProperty, &property) != nil || len(property) != 1 || len(property["type"]) == 0 {
			return ErrProtocol
		}
	}
	var schema Schema
	if len(raw) > 4096 || !utf8.Valid(raw) || decodeContext(raw, &schema) != nil || schema.Type != "object" || schema.Additional == nil || *schema.Additional || len(schema.Properties) != 4 || len(schema.Required) != 4 {
		return ErrProtocol
	}
	expected := map[string]string{"id": "string", "timeout_seconds": "integer", "unit": "string", "version": "string"}
	seen := map[string]bool{}
	for _, name := range schema.Required {
		kind, ok := expected[name]
		if !ok || seen[name] || schema.Properties[name].Type != kind {
			return ErrProtocol
		}
		seen[name] = true
	}
	return nil
}

func parseStructuredResult(raw []byte, id, version string) (StructuredDocument, error) {
	var fields map[string]json.RawMessage
	if len(raw) > 8192 || decodeContext(raw, &fields) != nil || len(fields["content"]) == 0 {
		return StructuredDocument{}, ErrProtocol
	}
	for name := range fields {
		if name != "content" && name != "structuredContent" && name != "isError" {
			return StructuredDocument{}, ErrProtocol
		}
	}
	var envelope structuredEnvelope
	if len(raw) > 8192 || !utf8.Valid(raw) || decodeContext(raw, &envelope) != nil {
		return StructuredDocument{}, ErrProtocol
	}
	if envelope.IsError {
		return StructuredDocument{}, ErrTool
	}
	if len(envelope.Content) != 1 || envelope.Content[0].Type != "text" || len(envelope.Content[0].Text) > 4096 || len(envelope.StructuredContent) == 0 {
		return StructuredDocument{}, ErrProtocol
	}
	var value, text StructuredDocument
	if decodeStructuredDocument(envelope.StructuredContent, &value) != nil || decodeStructuredDocument([]byte(envelope.Content[0].Text), &text) != nil || value != text || value.ID != id || value.Version != version || value.Unit != "seconds" || value.TimeoutSeconds < 1 || value.TimeoutSeconds > 3600 {
		return StructuredDocument{}, ErrProtocol
	}
	return value, nil
}

func decodeStructuredDocument(raw []byte, target *StructuredDocument) error {
	var fields map[string]json.RawMessage
	if len(raw) > 4096 || decodeContext(raw, &fields) != nil || len(fields) != 4 {
		return ErrProtocol
	}
	for _, name := range []string{"id", "timeout_seconds", "unit", "version"} {
		if _, ok := fields[name]; !ok {
			return ErrProtocol
		}
	}
	return decodeContext(raw, target)
}

// Observation projects validated fields only; the caller decides whether to provide it to a model.
func (d StructuredDocument) Observation() string {
	data, _ := json.Marshal(d)
	return string(data)
}

func structuredSuccess() structuredEnvelope {
	value := StructuredDocument{ID: "doc-A", TimeoutSeconds: 30, Unit: "seconds", Version: "4"}
	data, _ := json.Marshal(value)
	return structuredEnvelope{Content: []Content{{Type: "text", Text: string(data)}}, StructuredContent: data}
}

func (c *Client) CallStructured(ctx context.Context, name string, args map[string]any, expectedVersion string) (StructuredDocument, error) {
	if err := ctx.Err(); err != nil {
		return StructuredDocument{}, err
	}
	local := strings.TrimPrefix(name, c.alias+"::")
	if !c.ready || name == local || !c.approved[local] {
		return StructuredDocument{}, ErrDenied
	}
	definition, ok := c.tools[local]
	if !ok {
		return StructuredDocument{}, ErrDenied
	}
	if !arguments(args) || strings.TrimSpace(expectedVersion) == "" || len(expectedVersion) > 128 || !utf8.ValidString(expectedVersion) || validateStructuredSchema(definition.OutputSchema) != nil {
		return StructuredDocument{}, ErrProtocol
	}
	raw, err := c.exchange(ctx, "tools/call", map[string]any{"name": local, "arguments": args})
	if err != nil {
		return StructuredDocument{}, err
	}
	if err := ctx.Err(); err != nil {
		return StructuredDocument{}, err
	}
	value, err := parseStructuredResult(raw, args["id"].(string), expectedVersion)
	if err != nil {
		return StructuredDocument{}, err
	}
	if err := ctx.Err(); err != nil {
		return StructuredDocument{}, err
	}
	return value, nil
}
