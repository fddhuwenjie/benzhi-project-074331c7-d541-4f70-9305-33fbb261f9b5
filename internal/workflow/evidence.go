package workflow

import (
	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/validation"
)

// allRuleEvidencePassed 汇总当前校样沿用的最近规则证据。定向复验未覆盖的
// 规则可以沿用前版通过结论，但任何未被后续通过结果覆盖的失败都会阻止批准。
func allRuleEvidencePassed(a domain.Aggregate) bool {
	latest := a.LatestProof()
	if latest == nil {
		return false
	}
	results := make(map[string]bool, len(validation.AllRules))
	for _, run := range a.Runs {
		if run.ProofID == latest.ProofID {
			for _, result := range run.Results {
				results[result.RuleCode] = result.Passed
			}
			continue
		}
		// 相邻修订只会使 AffectedRules 返回的规则失效，其余通过证据可沿用。
		for _, result := range run.Results {
			if result.Passed {
				if _, exists := results[result.RuleCode]; !exists {
					results[result.RuleCode] = true
				}
			}
		}
	}
	for _, code := range validation.AllRules {
		if !results[code] {
			return false
		}
	}
	return true
}

func findingMayClose(a domain.Aggregate, finding domain.ReviewFinding) bool {
	if finding.CoverageStatus != "" {
		return finding.CoverageStatus == domain.CoverageClosable && finding.RemediationProofID != "" && finding.ValidationRunID != ""
	}
	proof := a.LatestProof()
	run := a.LatestRun()
	if proof == nil || run == nil || run.ProofID != proof.ProofID {
		return false
	}
	if finding.RuleCode == "MANUAL" {
		return run.AllPassed()
	}
	if proof.ProofID == finding.ProofID {
		return false
	}
	for _, result := range run.Results {
		if result.RuleCode == finding.RuleCode {
			return result.Passed
		}
	}
	return false
}
