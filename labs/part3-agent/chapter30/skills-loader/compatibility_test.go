package main

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestCompatibilityFieldHasIndependentFormatBound(t *testing.T) {
	base := "---\nname: test-skill\ndescription: Read evidence.\nmetadata:\n  version: v1\n"
	for _, value := range []string{strconv.Quote(strings.Repeat("x", 501)), `""`, `"   "`, "123", "null"} {
		if _, _, err := header([]byte(base + "compatibility: " + value + "\n---\nbody")); !errors.Is(err, ErrSkill) {
			t.Errorf("invalid compatibility accepted: bytes=%d err=%v", len(value), err)
		}
	}
	for _, field := range []string{"", "compatibility: " + strconv.Quote(strings.Repeat("界", 500)) + "\n"} {
		if _, _, err := header([]byte(base + field + "---\nbody")); err != nil {
			t.Fatal("omitted or valid 500-character boundary rejected", err)
		}
	}
}
