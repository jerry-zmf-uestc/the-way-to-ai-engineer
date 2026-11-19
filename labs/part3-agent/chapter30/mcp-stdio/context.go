package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"
)

const docURI = "course://docs/doc-A"

type Resource struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}
type ResourceText struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}
type PromptArgument struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments"`
}
type PromptMessage struct {
	Role    string  `json:"role"`
	Content Content `json:"content"`
}
type ContextCatalog struct {
	client                             *Client
	approvedResources, approvedPrompts map[string]bool
	resources                          map[string]Resource
	prompts                            map[string]Prompt
}

func uniqueContext(dec *json.Decoder, depth int) error {
	if depth > 16 {
		return ErrProtocol
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := []string{}
		for dec.More() {
			token, err = dec.Token()
			key, valid := token.(string)
			if err != nil || !valid {
				return ErrProtocol
			}
			for _, old := range seen {
				if strings.EqualFold(old, key) {
					return ErrProtocol
				}
			}
			seen = append(seen, key)
			if err = uniqueContext(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err = uniqueContext(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return ErrProtocol
	}
	_, err = dec.Token()
	return err
}
func decodeContext(data []byte, target any) error {
	if len(data) > 16<<10 || !utf8.Valid(data) || uniqueContext(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return ErrProtocol
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(target) != nil {
		return ErrProtocol
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return ErrProtocol
	}
	return nil
}
func fixedPrompt() Prompt {
	return Prompt{Name: "explain_doc", Description: "Prepare an optional explanation request from a fixed document.", Arguments: []PromptArgument{{Name: "id", Required: true}}}
}

func contextResponse(method string, params json.RawMessage, document string) (any, int, bool) {
	switch method {
	case "resources/list":
		var p map[string]any
		if decodeContext(params, &p) != nil || len(p) != 0 {
			return nil, -32602, true
		}
		return map[string]any{"resources": []Resource{{URI: docURI, Name: "doc-A", MimeType: "text/plain"}}}, 0, true
	case "resources/read":
		var p map[string]string
		if decodeContext(params, &p) != nil || len(p) != 1 || p["uri"] == "" {
			return nil, -32602, true
		}
		if p["uri"] != docURI {
			return nil, -32002, true
		}
		return map[string]any{"contents": []ResourceText{{URI: docURI, MimeType: "text/plain", Text: document}}}, 0, true
	case "prompts/list":
		var p map[string]any
		if decodeContext(params, &p) != nil || len(p) != 0 {
			return nil, -32602, true
		}
		return map[string]any{"prompts": []Prompt{fixedPrompt()}}, 0, true
	case "prompts/get":
		var p struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if decodeContext(params, &p) != nil || p.Name != "explain_doc" || len(p.Arguments) != 1 || p.Arguments["id"] != "doc-A" {
			return nil, -32602, true
		}
		return map[string]any{"description": "A user-selected explanation request.", "messages": []PromptMessage{{Role: "user", Content: Content{Type: "text", Text: "Explain the provided document. Document data: " + document}}}}, 0, true
	default:
		return nil, 0, false
	}
}
func NewContextCatalog(c *Client, resources, prompts map[string]bool) (*ContextCatalog, error) {
	if c == nil {
		return nil, ErrProtocol
	}
	copyPolicy := func(src map[string]bool) map[string]bool {
		out := map[string]bool{}
		for k, v := range src {
			out[k] = v
		}
		return out
	}
	return &ContextCatalog{client: c, approvedResources: copyPolicy(resources), approvedPrompts: copyPolicy(prompts), resources: map[string]Resource{}, prompts: map[string]Prompt{}}, nil
}
func (c *ContextCatalog) Discover(ctx context.Context) ([]Resource, []Prompt, error) {
	if !c.client.ready || !c.client.capabilities["resources"] || !c.client.capabilities["prompts"] {
		return nil, nil, ErrProtocol
	}
	raw, err := c.client.exchange(ctx, "resources/list", map[string]any{})
	if err != nil {
		return nil, nil, err
	}
	var r struct {
		Resources  []Resource `json:"resources"`
		NextCursor string     `json:"nextCursor"`
	}
	if decodeContext(raw, &r) != nil || len(r.Resources) > 8 || r.NextCursor != "" {
		return nil, nil, ErrProtocol
	}
	raw, err = c.client.exchange(ctx, "prompts/list", map[string]any{})
	if err != nil {
		return nil, nil, err
	}
	var p struct {
		Prompts    []Prompt `json:"prompts"`
		NextCursor string   `json:"nextCursor"`
	}
	if decodeContext(raw, &p) != nil || len(p.Prompts) > 8 || p.NextCursor != "" {
		return nil, nil, ErrProtocol
	}
	resources, prompts := map[string]Resource{}, map[string]Prompt{}
	outR, outP := []Resource{}, []Prompt{}
	for _, item := range r.Resources {
		uri, err := url.Parse(item.URI)
		if err != nil || uri.Scheme == "" || len(item.URI) > 512 || item.Name == "" || len(item.Name) > 128 || item.MimeType != "text/plain" {
			return nil, nil, ErrProtocol
		}
		if _, ok := resources[item.URI]; ok {
			return nil, nil, ErrProtocol
		}
		resources[item.URI] = item
		if c.approvedResources[item.URI] {
			outR = append(outR, item)
		}
	}
	for _, item := range p.Prompts {
		if item.Name == "" || len(item.Name) > 128 || len(item.Description) > 1024 || len(item.Arguments) != 1 || item.Arguments[0].Name != "id" || !item.Arguments[0].Required {
			return nil, nil, ErrProtocol
		}
		if _, ok := prompts[item.Name]; ok {
			return nil, nil, ErrProtocol
		}
		prompts[item.Name] = item
		if c.approvedPrompts[item.Name] {
			outP = append(outP, item)
		}
	}
	c.resources, c.prompts = resources, prompts
	return outR, outP, nil
}
func (c *ContextCatalog) Read(ctx context.Context, uri string) (string, error) {
	if !c.client.ready || !c.client.capabilities["resources"] || !c.approvedResources[uri] {
		return "", ErrDenied
	}
	if _, ok := c.resources[uri]; !ok {
		return "", ErrDenied
	}
	raw, err := c.client.exchange(ctx, "resources/read", map[string]string{"uri": uri})
	if err != nil {
		return "", err
	}
	var result struct {
		Contents []ResourceText `json:"contents"`
	}
	if decodeContext(raw, &result) != nil || len(result.Contents) != 1 {
		return "", ErrProtocol
	}
	part := result.Contents[0]
	if part.URI != uri || part.MimeType != "text/plain" || part.Text == "" || len(part.Text) > 4096 {
		return "", ErrProtocol
	}
	return part.Text, nil
}
func (c *ContextCatalog) GetPrompt(ctx context.Context, name, id string) ([]PromptMessage, error) {
	if !c.client.ready || !c.client.capabilities["prompts"] || !c.approvedPrompts[name] {
		return nil, ErrDenied
	}
	if _, ok := c.prompts[name]; !ok {
		return nil, ErrDenied
	}
	if id == "" || len(id) > 128 {
		return nil, ErrProtocol
	}
	raw, err := c.client.exchange(ctx, "prompts/get", map[string]any{"name": name, "arguments": map[string]string{"id": id}})
	if err != nil {
		return nil, err
	}
	var result struct {
		Description string          `json:"description"`
		Messages    []PromptMessage `json:"messages"`
	}
	if decodeContext(raw, &result) != nil || len(result.Description) > 1024 || len(result.Messages) < 1 || len(result.Messages) > 8 {
		return nil, ErrProtocol
	}
	bytes := 0
	for _, m := range result.Messages {
		bytes += len(m.Content.Text)
		if (m.Role != "user" && m.Role != "assistant") || m.Content.Type != "text" || m.Content.Text == "" || bytes > 4096 {
			return nil, ErrProtocol
		}
	}
	return result.Messages, nil
}
