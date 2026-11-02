package main

import (
	"errors"
	"image"
	"testing"
)

func TestDecodeFormatEmbeddedPNG(t *testing.T) {
	format, err := decodeFormat(tinyPNG)
	if err != nil {
		t.Fatalf("decodeFormat(tinyPNG) error = %v, want nil", err)
	}
	if format != "png" {
		t.Fatalf("decodeFormat(tinyPNG) = %q, want %q", format, "png")
	}
}

// 标准库中 image 包本身不默认注册任何格式：jpeg 与 gif 和 png 一样，
// 需要各自 import _ "image/jpeg" / import _ "image/gif" 才会注册。
// 本例只匿名导入了 image/png，因此 JPEG/GIF 魔数报 image: unknown format。
// （已用 Go 1.26 核实该行为。）
func TestJPEGNotRegistered(t *testing.T) {
	_, err := decodeFormat(jpegMagic)
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("decodeFormat(jpegMagic) error = %v, want image.ErrFormat", err)
	}
}

func TestGIFNotRegistered(t *testing.T) {
	_, err := decodeFormat(gifMagic)
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("decodeFormat(gifMagic) error = %v, want image.ErrFormat", err)
	}
}

func TestRegisteredFormats(t *testing.T) {
	got := registeredFormats()
	want := []string{"png"}
	if len(got) != len(want) {
		t.Fatalf("registeredFormats() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("registeredFormats() = %v, want %v（只匿名导入了 image/png）", got, want)
		}
	}
}
