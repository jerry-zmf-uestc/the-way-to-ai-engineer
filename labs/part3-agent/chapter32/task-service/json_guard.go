package main

import (
	"encoding/json"
	"errors"
	"strings"
)

var errAmbiguousJSON = errors.New("ambiguous or deep JSON")

func uniqueObjectFields(dec *json.Decoder, depth int) error {
	if depth > 16 {
		return errAmbiguousJSON
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errAmbiguousJSON
			}
			// Go struct decoding accepts case variants; two aliases must not overwrite.
			canonical := strings.ToLower(name)
			if seen[canonical] {
				return errAmbiguousJSON
			}
			seen[canonical] = true
			if err = uniqueObjectFields(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err = uniqueObjectFields(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return errAmbiguousJSON
	}
	_, err = dec.Token()
	return err
}
