package main

import (
	"errors"
	"os"
	"testing"
)

func TestLoadConfigWrapsNotFoundAsSentinel(t *testing.T) {
	_, err := LoadConfig("/definitely/missing/path")
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("LoadConfig() = %v, want errors.Is(ErrConfigNotFound)", err)
	}
}

func TestLoadConfigPropagatesUnderlyingError(t *testing.T) {
	_, err := LoadConfig("/definitely/missing/path")
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want error")
	}
	// 包装链上仍然能识别底层 os 错误，避免破坏调用方诊断能力。
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadConfig() = %v, want errors.Is(os.ErrNotExist)", err)
	}
}
