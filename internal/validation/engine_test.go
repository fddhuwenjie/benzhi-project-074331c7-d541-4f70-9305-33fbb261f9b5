package validation

import (
	"testing"

	"tactile-atlas-gate/internal/domain"
)

func validFixture() (domain.MapProject, domain.ProofRevision) {
	project := domain.MapProject{SheetWidthMM: 300, SheetHeightMM: 200, MinimumGapMM: 3}
	proof := domain.ProofRevision{
		Landmarks:      []domain.Landmark{{ID: "L1", X: 30, Y: 30}, {ID: "L2", X: 180, Y: 30}},
		PathSegments:   []domain.PathSegment{{ID: "P1", FromLandmarkID: "L1", ToLandmarkID: "L2"}},
		TactileSymbols: []domain.TactileSymbol{{ID: "S1", LegendKey: "entry", X: 30, Y: 30, RadiusMM: 4}, {ID: "S2", LegendKey: "hall", X: 180, Y: 30, RadiusMM: 4}},
		BrailleLabels:  []domain.BrailleLabel{{ID: "B1", Text: "入口", Cells: "⠁", X: 30, Y: 55, WidthMM: 12, HeightMM: 8}, {ID: "B2", Text: "展厅", Cells: "⠃", X: 180, Y: 55, WidthMM: 12, HeightMM: 8}},
		LegendEntries:  []domain.LegendEntry{{Key: "entry", Meaning: "入口"}, {Key: "hall", Meaning: "展厅"}},
	}
	return project, proof
}

func TestAllRulesPassForValidProof(t *testing.T) {
	project, proof := validFixture()
	results := NewEngine().Validate(project, proof, nil)
	seen := map[string]bool{}
	for _, result := range results {
		if !result.Passed {
			t.Fatalf("规则 %s 意外失败: %s", result.RuleCode, result.Message)
		}
		seen[result.RuleCode] = true
	}
	if len(seen) != len(AllRules) {
		t.Fatalf("规则覆盖数量为 %d，期望 %d", len(seen), len(AllRules))
	}
}

func TestInvalidProofReportsEveryRuleFamily(t *testing.T) {
	project, proof := validFixture()
	proof.BrailleLabels[0].Cells = "abc"
	proof.BrailleLabels[1].Text = proof.BrailleLabels[0].Text
	proof.BrailleLabels[0].X = 28
	proof.TactileSymbols[0].LegendKey = "missing"
	proof.TactileSymbols[1].X = 35
	proof.Landmarks[1].X = 301
	proof.PathSegments = nil
	failures := map[string]bool{}
	for _, result := range NewEngine().Validate(project, proof, nil) {
		if !result.Passed {
			failures[result.RuleCode] = true
		}
	}
	for _, code := range AllRules {
		if !failures[code] {
			t.Errorf("没有捕获规则失败: %s", code)
		}
	}
}

func TestAffectedRulesKeepsGlobalChecks(t *testing.T) {
	diff := domain.ProofDiff{ChangedSections: []string{"braille_labels"}}
	rules := AffectedRules(diff)
	seen := map[string]bool{}
	for _, code := range rules {
		seen[code] = true
	}
	for _, code := range []string{RuleBoundary, RuleConnectivity, RuleBraille, RuleDuplicate, RuleSpacing} {
		if !seen[code] {
			t.Errorf("定向复验缺少 %s", code)
		}
	}
}
