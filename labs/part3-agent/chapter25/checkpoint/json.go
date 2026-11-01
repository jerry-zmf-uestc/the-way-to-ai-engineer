package main

import (
	"encoding/json"
	"strings"
)

func uniqueStateJSON(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrState
	}
	token, err := dec.Token()
	if err != nil {
		return ErrState
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrState
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			token, err = dec.Token()
			key, ok := token.(string)
			if err != nil || !ok || seen[strings.ToLower(key)] {
				return ErrState
			}
			for _, r := range key {
				if r > 127 {
					return ErrState
				}
			}
			seen[strings.ToLower(key)] = true
		}
		if err := uniqueStateJSON(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrState
	}
	return nil
}
