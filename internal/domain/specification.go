package domain

import (
	"math"
	"sort"
	"strings"
	"time"
)

type SpecificationFieldResult struct {
	Field  string `json:"field"`
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

type SpecificationPreflight struct {
	Revision      int64                      `json:"revision"`
	Ready         bool                       `json:"ready"`
	Fields        []SpecificationFieldResult `json:"fields"`
	SpecDigest    string                     `json:"spec_digest"`
	SummaryDigest string                     `json:"summary_digest"`
	CheckedAt     time.Time                  `json:"checked_at"`
}

type FrozenSpecificationEvidence struct {
	ProjectID       string                 `json:"project_id"`
	VenueZone       string                 `json:"venue_zone"`
	SheetWidthMM    float64                `json:"sheet_width_mm"`
	SheetHeightMM   float64                `json:"sheet_height_mm"`
	MinimumGapMM    float64                `json:"minimum_gap_mm"`
	BrailleStandard string                 `json:"braille_standard"`
	ReviewerID      string                 `json:"reviewer_id"`
	SpecDigest      string                 `json:"spec_digest"`
	Preflight       SpecificationPreflight `json:"preflight"`
}

func FrozenSpecification(p MapProject, preflight SpecificationPreflight) FrozenSpecificationEvidence {
	return FrozenSpecificationEvidence{ProjectID: p.ProjectID, VenueZone: p.VenueZone, SheetWidthMM: p.SheetWidthMM, SheetHeightMM: p.SheetHeightMM, MinimumGapMM: p.MinimumGapMM, BrailleStandard: p.BrailleStandard, ReviewerID: p.ReviewerID, SpecDigest: preflight.SpecDigest, Preflight: preflight}
}

func PreflightSpecification(p MapProject, now time.Time) SpecificationPreflight {
	finitePositive := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }
	stablePrecision := func(v float64) bool { return math.Abs(v*1000-math.Round(v*1000)) < 1e-9 }
	short := math.Min(p.SheetWidthMM, p.SheetHeightMM)
	fields := []SpecificationFieldResult{
		{Field: "venue_zone", Valid: strings.TrimSpace(p.VenueZone) != "", Reason: "场馆区域不能为空"},
		{Field: "sheet_size", Valid: finitePositive(p.SheetWidthMM) && finitePositive(p.SheetHeightMM) && stablePrecision(p.SheetWidthMM) && stablePrecision(p.SheetHeightMM), Reason: "成品尺寸必须为有限正数且最多保留 3 位小数"},
		{Field: "minimum_gap_mm", Valid: finitePositive(p.MinimumGapMM) && stablePrecision(p.MinimumGapMM) && p.MinimumGapMM < short, Reason: "最小触觉间距必须为有限正数、最多保留 3 位小数且小于成品短边"},
		{Field: "braille_standard", Valid: p.BrailleStandard == "GB/T 15720" || p.BrailleStandard == "UEB", Reason: "盲文规范必须为 GB/T 15720 或 UEB"},
		{Field: "reviewer_id", Valid: strings.TrimSpace(p.ReviewerID) != "", Reason: "复核员不能为空"},
	}
	ready := true
	for i := range fields {
		if fields[i].Valid {
			fields[i].Reason = ""
		} else {
			ready = false
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
	spec, _ := SpecificationDigest(p)
	result := SpecificationPreflight{Revision: p.Revision, Ready: ready, Fields: fields, SpecDigest: spec, CheckedAt: now.UTC()}
	summary := struct {
		Revision   int64                      `json:"revision"`
		Ready      bool                       `json:"ready"`
		Fields     []SpecificationFieldResult `json:"fields"`
		SpecDigest string                     `json:"spec_digest"`
	}{result.Revision, result.Ready, result.Fields, result.SpecDigest}
	result.SummaryDigest, _ = Digest(summary)
	return result
}
