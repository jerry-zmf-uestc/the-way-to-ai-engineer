package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"os"
)

//go:embed comparison-cases.json
var comparisonCases []byte

//go:embed holdout-cases.json
var holdoutCases []byte

type ComparisonEvidence struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type ComparisonResource struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Text string `json:"text"`
}
type ComparisonInput struct {
	ID       string               `json:"id"`
	Question string               `json:"question"`
	Evidence []ComparisonEvidence `json:"evidence"`
	Modes    []ComparisonMode     `json:"modes"`
}
type ComparisonMode struct {
	Name      string               `json:"name"`
	Catalog   []Entry              `json:"catalog"`
	Skills    []Loaded             `json:"skills"`
	Resources []ComparisonResource `json:"resources"`
	UsedBytes int                  `json:"used_bytes"`
}

func exportComparisons(ctx context.Context, root *os.Root, output io.Writer, set string) error {
	data := comparisonCases
	switch set {
	case "development-v1":
	case "holdout-v1":
		data = holdoutCases
	default:
		return ErrSkill
	}
	var dataset struct {
		Version string            `json:"version"`
		Cases   []ComparisonInput `json:"cases"`
	}
	// Export uses an execution-only projection. Labels stay in the independent scorer file.
	if err := json.Unmarshal(data, &dataset); err != nil {
		return err
	}
	for i := range dataset.Cases {
		row := &dataset.Cases[i]
		for _, mode := range []string{"full", "selected"} {
			loader, err := New(root, 32<<10, map[string]bool{"incident-diagnosis": true, "report-format": true}, map[string]bool{"read-doc": true})
			if err != nil {
				return err
			}
			view := ComparisonMode{Name: mode, Catalog: loader.Catalog(), Skills: []Loaded{}, Resources: []ComparisonResource{}}
			names := loader.Select(row.Question)
			if mode == "full" {
				names = []string{}
				for _, entry := range view.Catalog {
					names = append(names, entry.Name)
				}
			}
			for _, name := range names {
				skill, err := loader.Load(ctx, name)
				if err != nil {
					return err
				}
				text, err := loader.ReadReference(ctx, name, "references/guide.md")
				if err != nil {
					return err
				}
				view.Skills = append(view.Skills, skill)
				view.Resources = append(view.Resources, ComparisonResource{Name: name, Path: "references/guide.md", Text: text})
			}
			view.UsedBytes = loader.Used
			row.Modes = append(row.Modes, view)
		}
	}
	return json.NewEncoder(output).Encode(struct {
		Version string            `json:"version"`
		Cases   []ComparisonInput `json:"cases"`
	}{dataset.Version, dataset.Cases})
}
