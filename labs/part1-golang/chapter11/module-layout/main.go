package main

import (
	"errors"
	"fmt"
	"strings"
)

type ProjectShape string

const (
	ShapePackage ProjectShape = "package"
	ShapeCommand ProjectShape = "command"
	ShapeMixed   ProjectShape = "mixed"
	ShapeServer  ProjectShape = "server"
)

var ErrUnknownShape = errors.New("unknown project shape")

type LayoutRecommendation struct {
	Shape       ProjectShape
	Summary     string
	Directories []string
	Reason      string
}

func RecommendLayout(shape ProjectShape) (LayoutRecommendation, error) {
	switch shape {
	case ShapePackage:
		return LayoutRecommendation{
			Shape:       shape,
			Summary:     "keep one importable package at the module root",
			Directories: []string{"go.mod", "<module>.go", "<module>_test.go"},
			Reason:      "a small public package is easier to import when the root package matches the module name",
		}, nil
	case ShapeCommand:
		return LayoutRecommendation{
			Shape:       shape,
			Summary:     "start with package main at the module root",
			Directories: []string{"go.mod", "main.go", "main_test.go"},
			Reason:      "a single command does not need cmd/ until the repository grows more binaries or packages",
		}, nil
	case ShapeMixed:
		return LayoutRecommendation{
			Shape:       shape,
			Summary:     "separate installable commands from importable packages",
			Directories: []string{"cmd/<name>/main.go", "<package>/", "internal/"},
			Reason:      "cmd/ makes binaries discoverable while internal/ keeps private support packages refactorable",
		}, nil
	case ShapeServer:
		return LayoutRecommendation{
			Shape:       shape,
			Summary:     "put server entrypoints in cmd/ and private logic in internal/",
			Directories: []string{"cmd/<service>/main.go", "internal/<domain>/", "go.mod"},
			Reason:      "servers are usually self-contained binaries, so private implementation belongs under internal/",
		}, nil
	default:
		return LayoutRecommendation{}, fmt.Errorf("%w: %s", ErrUnknownShape, shape)
	}
}

func FormatRecommendation(rec LayoutRecommendation) string {
	return fmt.Sprintf("%s: %s\nDirectories: %s\nWhy: %s",
		rec.Shape,
		rec.Summary,
		strings.Join(rec.Directories, ", "),
		rec.Reason,
	)
}

func main() {
	rec, err := RecommendLayout(ShapeServer)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(FormatRecommendation(rec))
}
