package domain

import (
	"sort"
	"strings"
)

type ValidationChangeCategory string

const (
	ValidationNewFailure        ValidationChangeCategory = "新增失败"
	ValidationPersistentFailure ValidationChangeCategory = "持续失败"
	ValidationResolved          ValidationChangeCategory = "转为通过"
	ValidationNotCovered        ValidationChangeCategory = "未被本次复验覆盖"
)

type ValidationChange struct {
	Category    ValidationChangeCategory `json:"category"`
	RuleCode    string                   `json:"rule_code"`
	ElementRefs []string                 `json:"element_refs"`
	Baseline    *RuleResult              `json:"baseline,omitempty"`
	Target      *RuleResult              `json:"target,omitempty"`
	BaselineAt  string                   `json:"baseline_at"`
	TargetAt    string                   `json:"target_at"`
}
type ValidationComparison struct {
	BaselineRunID         string             `json:"baseline_run_id"`
	TargetRunID           string             `json:"target_run_id"`
	EquivalentRuleVersion bool               `json:"equivalent_rule_version"`
	VersionWarning        string             `json:"version_warning,omitempty"`
	Changes               []ValidationChange `json:"changes"`
}

func ruleResultKey(r RuleResult) string {
	refs := append([]string(nil), r.ElementRefs...)
	sort.Strings(refs)
	return r.RuleCode + "\x00" + strings.Join(refs, "\x00")
}
func CompareValidationRuns(base, target ValidationRun) ValidationComparison {
	c := ValidationComparison{BaselineRunID: base.RunID, TargetRunID: target.RunID, EquivalentRuleVersion: base.RuleVersion == target.RuleVersion}
	if !c.EquivalentRuleVersion {
		c.VersionWarning = "规则版本不同，结果不可直接等价"
	}
	bm, tm := map[string]RuleResult{}, map[string]RuleResult{}
	covered := map[string]bool{}
	for _, r := range base.Results {
		bm[ruleResultKey(r)] = r
	}
	for _, r := range target.Results {
		tm[ruleResultKey(r)] = r
		covered[r.RuleCode] = true
	}
	for _, r := range target.Results {
		if !r.Passed {
			br, ok := bm[ruleResultKey(r)]
			cat := ValidationNewFailure
			if ok && !br.Passed {
				cat = ValidationPersistentFailure
			}
			tr := r
			c.Changes = append(c.Changes, ValidationChange{Category: cat, RuleCode: r.RuleCode, ElementRefs: r.ElementRefs, Baseline: optionalRule(br, ok), Target: &tr, BaselineAt: base.CreatedAt.Format(timeLayout), TargetAt: target.CreatedAt.Format(timeLayout)})
		}
	}
	for _, r := range base.Results {
		if r.Passed {
			continue
		}
		tr, ok := tm[ruleResultKey(r)]
		cat := ValidationNotCovered
		if covered[r.RuleCode] {
			if ok && tr.Passed {
				cat = ValidationResolved
			} else if !ok {
				for _, candidate := range target.Results {
					if candidate.RuleCode == r.RuleCode && candidate.Passed {
						tr, ok, cat = candidate, true, ValidationResolved
						break
					}
				}
			}
		}
		br := r
		c.Changes = append(c.Changes, ValidationChange{Category: cat, RuleCode: r.RuleCode, ElementRefs: r.ElementRefs, Baseline: &br, Target: optionalRule(tr, ok), BaselineAt: base.CreatedAt.Format(timeLayout), TargetAt: target.CreatedAt.Format(timeLayout)})
	}
	sort.SliceStable(c.Changes, func(i, j int) bool {
		a, b := c.Changes[i], c.Changes[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.RuleCode != b.RuleCode {
			return a.RuleCode < b.RuleCode
		}
		return strings.Join(a.ElementRefs, "\x00") < strings.Join(b.ElementRefs, "\x00")
	})
	return c
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func optionalRule(r RuleResult, ok bool) *RuleResult {
	if !ok {
		return nil
	}
	x := r
	return &x
}
