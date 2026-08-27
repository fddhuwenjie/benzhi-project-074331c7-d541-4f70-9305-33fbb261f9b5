package domain

import (
	"testing"
	"time"
)

func TestSpecificationPreflightReportsGapConflict(t *testing.T) {
	p := MapProject{VenueZone: "展厅", SheetWidthMM: 100, SheetHeightMM: 80, MinimumGapMM: 80, BrailleStandard: "GB/T 15720", ReviewerID: "reviewer", Revision: 4}
	result := PreflightSpecification(p, time.Now())
	if result.Ready {
		t.Fatal("间距等于成品短边时预检不应通过")
	}
	found := false
	for _, field := range result.Fields {
		if field.Field == "minimum_gap_mm" && !field.Valid {
			found = true
		}
	}
	if !found || result.SummaryDigest == "" || result.SpecDigest == "" {
		t.Fatal("预检未返回稳定的字段冲突与摘要")
	}
}

func TestProofPreflightReturnsAllReferenceErrors(t *testing.T) {
	p := ProofRevision{ProofID: "proof-1", ProjectID: "map-1", Sequence: 1, SourceDigest: "source-digest", SubmittedBy: "maker", Landmarks: []Landmark{{ID: "L1", Name: "入口", SymbolID: "missing-symbol", LabelID: "missing-label"}}, PathSegments: []PathSegment{{ID: "P1", FromLandmarkID: "L1", ToLandmarkID: "missing-point"}}, TactileSymbols: []TactileSymbol{{ID: "S1", LegendKey: "missing", RadiusMM: 1}}, BrailleLabels: []BrailleLabel{{ID: "B1", Text: "入口", Cells: "⠁", WidthMM: 2, HeightMM: 2}}, LegendEntries: []LegendEntry{{Key: "entry", Meaning: "入口"}}}
	result := PreflightProof(p)
	if result.Ready || len(result.Issues) != 4 {
		t.Fatalf("期望 4 个交叉引用错误，得到 %#v", result.Issues)
	}
}

func TestValidationComparisonDistinguishesResolvedAndNotCovered(t *testing.T) {
	now := time.Now()
	base := ValidationRun{RunID: "base", RuleVersion: "v1", CreatedAt: now, Results: []RuleResult{{RuleCode: "TACTILE_GAP", Passed: false, ElementRefs: []string{"S1", "S2"}, Message: "间距失败"}, {RuleCode: "BRAILLE_CELLS", Passed: false, ElementRefs: []string{"B1"}, Message: "盲文失败"}}}
	target := ValidationRun{RunID: "target", RuleVersion: "v1", CreatedAt: now.Add(time.Minute), Targeted: true, Results: []RuleResult{{RuleCode: "TACTILE_GAP", Passed: true, Message: "间距通过"}}}
	comparison := CompareValidationRuns(base, target)
	categories := map[string]ValidationChangeCategory{}
	for _, change := range comparison.Changes {
		categories[change.RuleCode] = change.Category
	}
	if categories["TACTILE_GAP"] != ValidationResolved || categories["BRAILLE_CELLS"] != ValidationNotCovered {
		t.Fatalf("检查变化分类错误: %#v", categories)
	}
}
