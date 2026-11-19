package main

import (
	"errors"
	"strings"
	"testing"
)

func TestRecommendLayout(t *testing.T) {
	tests := []struct {
		name       string
		shape      ProjectShape
		wantDir    string
		wantReason string
	}{
		{
			name:       "basic package stays at root",
			shape:      ShapePackage,
			wantDir:    "<module>.go",
			wantReason: "root package",
		},
		{
			name:       "mixed repository uses cmd",
			shape:      ShapeMixed,
			wantDir:    "cmd/<name>/main.go",
			wantReason: "internal/",
		},
		{
			name:       "server keeps implementation private",
			shape:      ShapeServer,
			wantDir:    "internal/<domain>/",
			wantReason: "self-contained binaries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RecommendLayout(tt.shape)
			if err != nil {
				t.Fatalf("RecommendLayout(%q) error = %v", tt.shape, err)
			}

			if !contains(got.Directories, tt.wantDir) {
				t.Fatalf("directories = %v, want %q", got.Directories, tt.wantDir)
			}
			if !strings.Contains(got.Reason, tt.wantReason) {
				t.Fatalf("reason = %q, want it to mention %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestRecommendLayoutUnknownShape(t *testing.T) {
	_, err := RecommendLayout(ProjectShape("workspace"))
	if !errors.Is(err, ErrUnknownShape) {
		t.Fatalf("error = %v, want ErrUnknownShape", err)
	}
}

func TestFormatRecommendation(t *testing.T) {
	rec, err := RecommendLayout(ShapeCommand)
	if err != nil {
		t.Fatalf("RecommendLayout(%q) error = %v", ShapeCommand, err)
	}

	got := FormatRecommendation(rec)
	for _, want := range []string{"command:", "Directories:", "Why:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted recommendation = %q, want it to contain %q", got, want)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
