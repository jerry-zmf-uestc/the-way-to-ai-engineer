package main

import (
	"encoding/json"
	"errors"
	"unicode/utf8"
)

var (
	ErrToolInputLimit = errors.New("tool calling: input size or encoding rejected")
	ErrToolResult     = errors.New("tool calling: successful result size or encoding rejected")
)

func definitionFits(tool Tool) bool {
	if len(tool.Name) > 128 || !utf8.ValidString(tool.Name) || len(tool.Description) > 1024 || !utf8.ValidString(tool.Description) || len(tool.Parameters.Properties) > 32 || len(tool.Parameters.Required) > 32 {
		return false
	}
	for name := range tool.Parameters.Properties {
		if len(name) > 128 || !utf8.ValidString(name) {
			return false
		}
	}
	return true
}

func payloadFits(name string, args map[string]any) error {
	if len(name) > 128 || !utf8.ValidString(name) || len(args) > 32 {
		return ErrToolInputLimit
	}
	size := len(name)
	for key, value := range args {
		if len(key) > 128 || !utf8.ValidString(key) {
			return ErrToolInputLimit
		}
		size += len(key) + 32
		if text, ok := value.(string); ok {
			if len(text) > 4096 || !utf8.ValidString(text) {
				return ErrToolInputLimit
			}
			size += len(text)
		}
		if size > 16<<10 {
			return ErrToolInputLimit
		}
	}
	return nil
}

// Called only after scalar type validation; the raw precheck bounds JSON escaping allocation.
func encodedArgsFit(args map[string]any) error {
	data, err := json.Marshal(args)
	if err != nil || len(data) > 16<<10 {
		return ErrToolInputLimit
	}
	return nil
}

func resultFits(text string) bool {
	return len(text) <= 4096 && utf8.ValidString(text)
}
