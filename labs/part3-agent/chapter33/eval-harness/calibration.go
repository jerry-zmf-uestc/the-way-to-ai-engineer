package main

import "sort"

type Assessment struct{ ID, Stratum, Reference, Judged string }
type CalibrationCounts struct {
	// Matrix rows are reference labels; columns are Judge labels.
	// Both use supported, unsupported, unknown in that order.
	Matrix                                                              [3][3]int
	Total, ReferenceKnown, ReferenceUnknown, JudgedKnown, Agree         int
	SupportedReference, UnsupportedReference, FalseSupport, FalseReject int
}
type CalibrationGroup struct {
	Stratum string
	Counts  CalibrationCounts
}
type CalibrationReport struct {
	DatasetVersion, ReferenceVersion, RubricVersion, JudgeVersion string
	Total                                                         CalibrationCounts
	Groups                                                        []CalibrationGroup
}
type Ratio struct {
	Value float64
	Known bool
}
type CalibrationRates struct {
	Agreement, Coverage, FalseSupport, FalseReject Ratio
}

func fraction(numerator, denominator int) Ratio {
	if denominator == 0 {
		return Ratio{}
	}
	return Ratio{Value: float64(numerator) / float64(denominator), Known: true}
}
func (c CalibrationCounts) Rates() CalibrationRates {
	return CalibrationRates{
		Agreement:    fraction(c.Agree, c.ReferenceKnown),
		Coverage:     fraction(c.JudgedKnown, c.ReferenceKnown),
		FalseSupport: fraction(c.FalseSupport, c.UnsupportedReference),
		FalseReject:  fraction(c.FalseReject, c.SupportedReference),
	}
}

func (c *CalibrationCounts) add(reference, judged int) {
	c.Total++
	c.Matrix[reference][judged]++
	if reference == 2 {
		c.ReferenceUnknown++
		return
	}
	c.ReferenceKnown++
	if judged != 2 {
		c.JudgedKnown++
	}
	if reference == judged {
		c.Agree++
	}
	if reference == 0 {
		c.SupportedReference++
		if judged == 1 {
			c.FalseReject++
		}
	} else {
		c.UnsupportedReference++
		if judged == 0 {
			c.FalseSupport++
		}
	}
}

// Calibration diagnoses agreement with supplied references; it cannot prove their validity.
func Calibrate(dataset, reference, rubric, judge string, rows []Assessment) (CalibrationReport, error) {
	for _, version := range []string{dataset, reference, rubric, judge} {
		if version == "" || len(version) > 128 {
			return CalibrationReport{}, ErrInvalid
		}
	}
	if len(rows) < 1 || len(rows) > 1000 {
		return CalibrationReport{}, ErrInvalid
	}
	positions := map[string]int{"supported": 0, "unsupported": 1, "unknown": 2}
	seen := map[string]bool{}
	groups := map[string]CalibrationCounts{}
	report := CalibrationReport{DatasetVersion: dataset, ReferenceVersion: reference, RubricVersion: rubric, JudgeVersion: judge}
	for _, row := range rows {
		r, referenceOK := positions[row.Reference]
		j, judgedOK := positions[row.Judged]
		if row.ID == "" || len(row.ID) > 128 || seen[row.ID] || row.Stratum == "" || len(row.Stratum) > 128 || !referenceOK || !judgedOK {
			return CalibrationReport{}, ErrInvalid
		}
		seen[row.ID] = true
		report.Total.add(r, j)
		group := groups[row.Stratum]
		group.add(r, j)
		groups[row.Stratum] = group
	}
	for stratum, counts := range groups {
		report.Groups = append(report.Groups, CalibrationGroup{stratum, counts})
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Stratum < report.Groups[j].Stratum })
	return report, nil
}

func calibrationDemo() []Assessment {
	return []Assessment{
		{"normal1", "normal", "supported", "supported"},
		{"normal2", "normal", "supported", "supported"},
		{"normal3", "normal", "supported", "supported"},
		{"risk1", "risk", "unsupported", "supported"},
		{"risk2", "risk", "unsupported", "unknown"},
		{"ambiguous1", "ambiguous", "unknown", "supported"},
	}
}
