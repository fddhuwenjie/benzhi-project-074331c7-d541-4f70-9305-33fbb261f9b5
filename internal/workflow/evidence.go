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
	present := make(map[string]bool, len(validation.AllRules))
	// 按校样与运行的时间顺序汇总：每条规则以最近一次覆盖它的结果为准，较新的
	// 失败不会被更早的通过覆盖，未被最新定向复验覆盖的规则沿用其最近结论。
	// 这与 approvalRuleEvidence 对同一证据的计算保持一致。
	for _, run := range a.Runs {
		for _, result := range run.Results {
			results[result.RuleCode] = result.Passed
			present[result.RuleCode] = true
		}
	}
	for _, code := range validation.AllRules {
		if !present[code] || !results[code] {
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
