package main

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryJSONRejectsAmbiguityAndBadShape(t *testing.T) {
	valid, err := json.Marshal(summaryExample())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSummary(valid)
	if err != nil || !reflect.DeepEqual(got, summaryExample()) {
		t.Fatalf("valid summary: %+v %v", got, err)
	}
	for _, raw := range [][]byte{
		[]byte(strings.Replace(string(valid), `"Version":"task-v2"`, `"Version":"task-v1","Version":"task-v2"`, 1)),
		[]byte(strings.Replace(string(valid), `"Version"`, `"version"`, 1)),
		[]byte(strings.Replace(string(valid), `"Value":"6000"`, `"Value":6000`, 1)),
		[]byte(strings.Replace(string(valid), `"Value":"6000"`, `"Value":null`, 1)),
		[]byte(strings.Replace(string(valid), `"State":"known"`, `"State":"unknown","State":"known"`, 1)),
		[]byte(strings.Replace(string(valid), `"Key":"budget_cny"`, `"Key":"budget_cny","Extra":true`, 1)),
		[]byte(string(valid) + `{}`), []byte("```json\n" + string(valid) + "\n```"),
		[]byte(`{"Version":"task-v2","Facts":null}`), []byte(`{"Version":null,"Facts":[]}`),
		[]byte(`{"Version":"task-v2","Facts":[[[[[[{}]]]]]]}`),
		[]byte(strings.Repeat(" ", 64<<10) + string(valid)),
		append([]byte(`{"Version":"`), 0xff),
	} {
		got, err := ParseSummary(raw)
		if !errors.Is(err, ErrSummary) || got.Version != "" || len(got.Facts) != 0 {
			t.Fatalf("bad raw accepted: %+v %v", got, err)
		}
	}
}

func TestActualCompressionRecordsAreNotPublished(t *testing.T) {
	for _, mode := range []string{"full", "recent"} {
		raw, err := os.ReadFile("testdata/compression-development-v1-result-" + mode + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseSummary(raw)
		if !errors.Is(err, ErrSummary) || len(got.Facts) != 0 {
			t.Fatalf("actual %s candidate published: %+v %v", mode, got, err)
		}
	}
}
