package validation_result_cache_alias_test

import (
	"testing"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/validation"
)

func TestCachedValidationResultIsIsolatedFromCallerMutation(t *testing.T) {
	engine := validation.NewEngine()
	project := domain.MapProject{
		ProjectID:     "cache-project",
		SheetWidthMM:  100,
		SheetHeightMM: 100,
		MinimumGapMM:  3,
	}
	proof := domain.ProofRevision{
		ProofID:   "cache-proof",
		ProjectID: project.ProjectID,
		TactileSymbols: []domain.TactileSymbol{
			{ID: "outside-symbol", X: 2, Y: 2, RadiusMM: 4},
		},
	}

	first := engine.Validate(project, proof, []string{validation.RuleBoundary})
	if len(first) != 1 || first[0].Passed || len(first[0].ElementRefs) != 1 {
		t.Fatalf("测试夹具未产生预期的边界失败: %#v", first)
	}
	first[0].Passed = true
	first[0].Message = "调用方改写"
	first[0].ElementRefs[0] = "polluted-ref"

	second := engine.Validate(project, proof, []string{validation.RuleBoundary})
	if len(second) != 1 || second[0].Passed || second[0].Message != "触觉符号超出成品边界" || len(second[0].ElementRefs) != 1 || second[0].ElementRefs[0] != "outside-symbol" {
		t.Fatalf("缓存结果被上一次调用方污染: %#v", second)
	}
}
