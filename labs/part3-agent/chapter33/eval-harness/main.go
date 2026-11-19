package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"sort"
	"time"
)

//go:embed fixtures.json
var fixtureBytes []byte
var ErrInvalid = errors.New("eval: invalid dataset or result")

type Case struct {
	ID, Split, Input                                       string
	Relevant, AllowedSources, AllowedTools, RequiredClaims []string
	ExpectAbstain                                          bool
}
type Claim struct {
	Text    string
	Sources []string
}
type Result struct {
	CaseID, RunID, Error       string
	Retrieved, Evidence, Tools []string
	Claims                     []Claim
	Abstained                  bool
	Tokens                     int
	TokensKnown                bool
}
type Label struct{ CaseID, Text, Source, Verdict string }
type labelKey struct{ CaseID, Text, Source string }
type Row struct {
	Split                             string
	CaseID, Status                    string
	Recall                            float64
	RecallKnown                       bool
	Links, KnownLinks, SupportedLinks int
	Tokens                            int
	TokensKnown                       bool
}
type Report struct {
	SupportKnown                                bool
	Links, KnownLinks, SupportedLinks           int
	DatasetVersion, Config, ScorerVersion       string
	Total, Passed, Failed, Review, UnknownUsage int
	SupportRate, SupportCoverage, MeanRecall    float64
	RecallCases                                 int
	Tokens                                      int
	Wilson95                                    [2]float64
	Rows                                        []Row
}
type Fixture struct {
	Version             string
	Cases               []Case
	Labels              []Label
	Baseline, Candidate []Result
}

func caseIDs(cases []Case) (map[string]bool, error) {
	if len(cases) == 0 || len(cases) > 100 {
		return nil, ErrInvalid
	}
	ids := map[string]bool{}
	for _, c := range cases {
		if c.ID == "" || c.Input == "" || len(c.Input) > 4096 || ids[c.ID] || (c.Split != "development" && c.Split != "regression" && c.Split != "adversarial" && c.Split != "production-like") {
			return nil, ErrInvalid
		}
		ids[c.ID] = true
	}
	return ids, nil
}

// Runner绑定Case身份与每Case截止时间；usage来自执行记录，不由模型填写。
// 同步Task须协作响应context，本Runner不抢占不合作函数。
type TaskInput struct {
	ID, Input                    string
	AllowedSources, AllowedTools []string
}
type Task func(context.Context, TaskInput) (Result, error)

func RunCases(ctx context.Context, cases []Case, timeout time.Duration, task Task) ([]Result, error) {
	if len(cases) == 0 || len(cases) > 100 || timeout <= 0 || task == nil {
		return nil, ErrInvalid
	}
	if _, err := caseIDs(cases); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(cases))
	for n, c := range cases {
		result := Result{CaseID: c.ID, RunID: fmt.Sprintf("%s#%d", c.ID, n)}
		if ctx.Err() != nil {
			result.Error = "canceled_before_start"
			results = append(results, result)
			continue
		}
		child, cancel := context.WithTimeout(ctx, timeout)
		input := TaskInput{ID: c.ID, Input: c.Input, AllowedSources: append([]string(nil), c.AllowedSources...), AllowedTools: append([]string(nil), c.AllowedTools...)}
		actual, err := task(child, input)
		if child.Err() != nil {
			err = child.Err()
		}
		cancel()
		actual.CaseID = result.CaseID
		actual.RunID = result.RunID
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				actual.Error = "canceled"
			case errors.Is(err, context.DeadlineExceeded):
				actual.Error = "deadline_exceeded"
			default:
				actual.Error = "execution_failed"
			}
		}
		results = append(results, actual)
	}
	return results, nil
}
func Load(data []byte) (Fixture, error) {
	if len(data) > 256<<10 {
		return Fixture{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f Fixture
	if err := dec.Decode(&f); err != nil {
		return Fixture{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Fixture{}, ErrInvalid
	}
	return f, nil
}
func recall(relevant, retrieved []string) float64 {
	found := map[string]bool{}
	for _, id := range retrieved {
		found[id] = true
	}
	unique := map[string]bool{}
	for _, id := range relevant {
		unique[id] = true
	}
	n := 0
	for id := range unique {
		if found[id] {
			n++
		}
	}
	if len(unique) == 0 {
		return 0
	}
	return float64(n) / float64(len(unique))
}

// score的标签来自评估数据，不接受模型自报supported。未知标签不补为成功。
func score(c Case, r Result, labels map[labelKey]string) Row {
	row := Row{Split: c.Split, CaseID: c.ID, Status: "passed", Tokens: r.Tokens, TokensKnown: r.TokensKnown}
	if len(c.Relevant) > 0 {
		row.RecallKnown = true
		row.Recall = recall(c.Relevant, r.Retrieved)
	}
	if r.Error != "" {
		row.Status = "failed"
		return row
	}
	for _, name := range r.Tools {
		if !slices.Contains(c.AllowedTools, name) {
			row.Status = "failed"
		}
	}
	if r.Abstained != c.ExpectAbstain {
		row.Status = "failed"
	}
	if c.ExpectAbstain {
		if len(r.Claims) > 0 {
			row.Status = "failed"
		}
		return row
	}
	if len(r.Claims) == 0 {
		row.Status = "failed"
	}
	present := map[string]bool{}
	needsReview := false
	for _, claim := range r.Claims {
		present[claim.Text] = true
		if len(claim.Sources) == 0 {
			row.Status = "failed"
		}
		seen := map[string]bool{}
		for _, source := range claim.Sources {
			if seen[source] {
				continue
			}
			seen[source] = true
			row.Links++
			if !slices.Contains(c.AllowedSources, source) || !slices.Contains(r.Evidence, source) {
				row.Status = "failed"
				continue
			}
			verdict, ok := labels[labelKey{c.ID, claim.Text, source}]
			if !ok || verdict == "unknown" {
				needsReview = true
				continue
			}
			row.KnownLinks++
			if verdict == "supported" {
				row.SupportedLinks++
			} else {
				row.Status = "failed"
			}
		}
	}
	for _, required := range c.RequiredClaims {
		if !present[required] {
			row.Status = "failed"
		}
	}
	if row.Status == "passed" && needsReview {
		row.Status = "review"
	}
	return row
}

// Evaluate保留全部Case分母。缺失结果产生失败行，不从平均分中消失。
func Evaluate(version, config string, cases []Case, results []Result, annotations []Label) (Report, error) {
	if version == "" || config == "" || len(cases) == 0 || len(cases) > 100 || len(results) > 100 || len(annotations) > 1000 {
		return Report{}, ErrInvalid
	}
	known, err := caseIDs(cases)
	if err != nil {
		return Report{}, err
	}
	byID := map[string]Result{}
	runs := map[string]bool{}
	for _, r := range results {
		if !known[r.CaseID] || r.RunID == "" || runs[r.RunID] || r.Tokens < 0 || len(r.Claims) > 16 || len(r.Retrieved) > 128 || len(r.Tools) > 128 {
			return Report{}, ErrInvalid
		}
		if _, ok := byID[r.CaseID]; ok {
			return Report{}, ErrInvalid
		}
		byID[r.CaseID] = r
		runs[r.RunID] = true
	}
	labels := map[labelKey]string{}
	for _, l := range annotations {
		key := labelKey{l.CaseID, l.Text, l.Source}
		if !known[l.CaseID] || l.Text == "" || l.Source == "" || (l.Verdict != "supported" && l.Verdict != "unsupported" && l.Verdict != "unknown") {
			return Report{}, ErrInvalid
		}
		if _, ok := labels[key]; ok {
			return Report{}, ErrInvalid
		}
		labels[key] = l.Verdict
	}
	report := Report{DatasetVersion: version, Config: config, ScorerVersion: "rules-v1", Total: len(cases)}
	totalLinks, knownLinks, supportedLinks := 0, 0, 0
	for _, c := range cases {
		result, ok := byID[c.ID]
		if !ok {
			result = Result{CaseID: c.ID, Error: "missing_result"}
		}
		row := score(c, result, labels)
		report.Rows = append(report.Rows, row)
		switch row.Status {
		case "passed":
			report.Passed++
		case "failed":
			report.Failed++
		case "review":
			report.Review++
		}
		totalLinks += row.Links
		knownLinks += row.KnownLinks
		supportedLinks += row.SupportedLinks
		if row.RecallKnown {
			report.MeanRecall += row.Recall
			report.RecallCases++
		}
		if row.TokensKnown {
			if row.Tokens > int(^uint(0)>>1)-report.Tokens {
				return Report{}, ErrInvalid
			}
			report.Tokens += row.Tokens
		} else {
			report.UnknownUsage++
		}
	}
	report.Links = totalLinks
	report.KnownLinks = knownLinks
	report.SupportedLinks = supportedLinks
	if knownLinks > 0 {
		report.SupportKnown = true
		report.SupportRate = float64(supportedLinks) / float64(knownLinks)
	}
	if totalLinks > 0 {
		report.SupportCoverage = float64(knownLinks) / float64(totalLinks)
	}
	if report.RecallCases > 0 {
		report.MeanRecall /= float64(report.RecallCases)
	}
	report.Wilson95 = wilson(report.Passed, report.Total)
	return report, nil
}
func wilson(success, total int) [2]float64 {
	n := float64(total)
	p := float64(success) / n
	z := 1.96
	denom := 1 + z*z/n
	center := (p + z*z/(2*n)) / denom
	half := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denom
	return [2]float64{math.Max(0, center-half), math.Min(1, center+half)}
}

// Regressions要求同数据集、评分器和Case集合；review不能当成passed。
func Regressions(base, next Report) ([]string, error) {
	if base.DatasetVersion == "" || base.ScorerVersion == "" || len(base.Rows) == 0 || base.DatasetVersion != next.DatasetVersion || base.ScorerVersion != next.ScorerVersion || len(base.Rows) != len(next.Rows) {
		return nil, ErrInvalid
	}
	byID := map[string]string{}
	for _, r := range next.Rows {
		if r.CaseID == "" || (r.Status != "passed" && r.Status != "failed" && r.Status != "review") {
			return nil, ErrInvalid
		}
		if _, exists := byID[r.CaseID]; exists {
			return nil, ErrInvalid
		}
		byID[r.CaseID] = r.Status
	}
	out := []string{}
	baseIDs := map[string]bool{}
	for _, r := range base.Rows {
		if r.CaseID == "" || baseIDs[r.CaseID] || (r.Status != "passed" && r.Status != "failed" && r.Status != "review") {
			return nil, ErrInvalid
		}
		baseIDs[r.CaseID] = true
		status, ok := byID[r.CaseID]
		if !ok {
			return nil, ErrInvalid
		}
		if r.Status == "passed" && status != "passed" {
			out = append(out, r.CaseID)
		}
	}
	sort.Strings(out)
	return out, nil
}
func main() {
	calibration := flag.Bool("calibration", false, "show synthetic reference/Judge diagnostic counts")
	coverage := flag.String("coverage", "", "score a local Schema1 evidence coverage file")
	flag.Parse()
	coverageSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "coverage" {
			coverageSet = true
		}
	})
	if flag.NArg() != 0 || *calibration && coverageSet || coverageSet && *coverage == "" {
		fmt.Fprintln(os.Stderr, ErrInvalid)
		os.Exit(2)
	}
	if *coverage != "" {
		file, err := os.Open(*coverage)
		if err != nil {
			fmt.Fprintln(os.Stderr, ErrInvalid)
			os.Exit(2)
		}
		report, readErr := LoadCoverage(file)
		if err = errors.Join(readErr, file.Close()); err != nil {
			fmt.Fprintln(os.Stderr, ErrInvalid)
			os.Exit(2)
		}
		data, err := json.Marshal(CoverageCLIReport{Schema: 1, Report: report})
		if err != nil || len(data)+1 > 512<<10 {
			fmt.Fprintln(os.Stderr, ErrInvalid)
			os.Exit(2)
		}
		if n, err := os.Stdout.Write(append(data, '\n')); err != nil || n != len(data)+1 {
			os.Exit(2)
		}
		return
	}
	if *calibration {
		report, err := Calibrate("synthetic-v1", "author-demo-v1", "support-v1", "fixed-demo-v1", calibrationDemo())
		if err != nil {
			panic(err)
		}
		rates := report.Total.Rates()
		fmt.Printf("synthetic=true reference_known=%d reference_unknown=%d agreement=%.2f coverage=%.2f false_support=%.2f\n", report.Total.ReferenceKnown, report.Total.ReferenceUnknown, rates.Agreement.Value, rates.Coverage.Value, rates.FalseSupport.Value)
		for _, group := range report.Groups {
			fmt.Printf("stratum=%s total=%d matrix=%v rates=%+v\n", group.Stratum, group.Counts.Total, group.Counts.Matrix, group.Counts.Rates())
		}
		return
	}
	f, err := Load(fixtureBytes)
	if err != nil {
		panic(err)
	}
	base, err := Evaluate(f.Version, "baseline", f.Cases, f.Baseline, f.Labels)
	if err != nil {
		panic(err)
	}
	next, err := Evaluate(f.Version, "candidate", f.Cases, f.Candidate, f.Labels)
	if err != nil {
		panic(err)
	}
	regressions, err := Regressions(base, next)
	if err != nil {
		panic(err)
	}
	fmt.Printf("baseline passed=%d/%d candidate passed=%d/%d failed=%d review=%d\n", base.Passed, base.Total, next.Passed, next.Total, next.Failed, next.Review)
	rate := "unscored"
	if next.SupportKnown {
		rate = fmt.Sprintf("%.2f", next.SupportRate)
	}
	fmt.Printf("support_rate=%s coverage=%.2f mean_recall=%.2f unknown_usage=%d regressions=%v\n", rate, next.SupportCoverage, next.MeanRecall, next.UnknownUsage, regressions)
}
