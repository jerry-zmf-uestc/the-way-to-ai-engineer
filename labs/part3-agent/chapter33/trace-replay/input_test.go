package main

import (
	"errors"
	"strings"
	"testing"
)

func TestArchiveAmbiguousJSONIsRejected(t *testing.T) {
	for _, raw := range []string{
		`{"Schema":0,"Schema":1,"Run":"demo","Calls":[]}`,
		`{"Schema":1,"Run":"demo","run":"other","Calls":[]}`,
		`{"Schema":0,"ſchema":1,"Run":"demo","Calls":[]}`,
		`{"Schema":1,"Run":"demo","Calls":[[[[[[[[[[]]]]]]]]]]}`,
		`{"Schema":1,"Run":"demo","Calls":[]} {}`,
		"{\"Schema\":1,\"Run\":\"" + string([]byte{0xff}) + "\",\"Calls\":[]}",
	} {
		a, err := Load([]byte(raw))
		if !errors.Is(err, ErrTrace) || a.Run != "" {
			t.Fatalf("ambiguous archive accepted: %+v %v", a, err)
		}
	}
}

func TestArchiveInputGuardPreservesValidUnicodeValues(t *testing.T) {
	a, err := Load([]byte(`{"Schema":1,"Run":"课程运行","Calls":[]}`))
	if err != nil || a.Run != "课程运行" {
		t.Fatal(a, err)
	}
	if _, err := Load([]byte(strings.Repeat(" ", 80<<10) + `{"Schema":1}`)); !errors.Is(err, ErrTrace) {
		t.Fatal("oversized raw bytes accepted", err)
	}
}
