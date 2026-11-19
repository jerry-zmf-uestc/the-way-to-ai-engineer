package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

func LocalDataset(name string) ([]Case, map[string]Document, error) {
	if name == "fixture-v1" {
		cases, docs := fixtures()
		return cases, docs, nil
	}
	if name != "independent-v1" && name != "holdout-v3" && name != "holdout-v4" {
		return nil, nil, ErrInput
	}
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", name+".json"))
	if err != nil {
		return nil, nil, err
	}
	var spec struct {
		Version string
		Cases   []Case
		Corpus  map[string]Document
	}
	if json.Unmarshal(data, &spec) != nil || spec.Version != name || len(spec.Cases) != 6 || len(spec.Corpus) > 32 {
		return nil, nil, ErrInput
	}
	return spec.Cases, spec.Corpus, nil
}
