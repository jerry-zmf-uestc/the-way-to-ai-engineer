package main

import "encoding/json"
import "strings"

func uniqueArchiveJSON(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrTrace
	}
	token, err := dec.Token()
	if err != nil {
		return ErrTrace
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrTrace
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			token, err = dec.Token()
			key, ok := token.(string)
			if err != nil || !ok || seen[strings.ToLower(key)] {
				return ErrTrace
			}
			for _, r := range key {
				if r > 127 {
					return ErrTrace
				}
			}
			seen[strings.ToLower(key)] = true
		}
		if err := uniqueArchiveJSON(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrTrace
	}
	return nil
}
