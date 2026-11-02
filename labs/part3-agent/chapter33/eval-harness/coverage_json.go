package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

type CoverageFile struct {
	Schema                             int
	DatasetVersion, RequirementVersion string
	Cases                              []Case
	Requirements                       []EvidenceRequirement
	Runs                               []CoverageRun
}
type CoverageCLIReport struct {
	Schema int
	Report CoverageReport
}

func uniqueCoverageJSON(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrInvalid
	}
	token, err := dec.Token()
	if err != nil {
		return ErrInvalid
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			token, err = dec.Token()
			key, ok := token.(string)
			if err != nil || !ok || seen[strings.ToLower(key)] {
				return ErrInvalid
			}
			for _, r := range key {
				if r > 127 {
					return ErrInvalid
				}
			}
			seen[strings.ToLower(key)] = true
		}
		if err := uniqueCoverageJSON(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrInvalid
	}
	return nil
}

func LoadCoverage(reader io.Reader) (CoverageReport, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (256<<10)+1))
	if err != nil || len(data) > 256<<10 || !utf8.Valid(data) || uniqueCoverageJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return CoverageReport{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f CoverageFile
	if dec.Decode(&f) != nil || dec.Decode(new(any)) != io.EOF || f.Schema != 1 {
		return CoverageReport{}, ErrInvalid
	}
	return ScoreEvidenceCoverage(f.DatasetVersion, f.RequirementVersion, f.Cases, f.Requirements, f.Runs)
}
