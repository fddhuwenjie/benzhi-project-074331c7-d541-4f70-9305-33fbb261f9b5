package validation

import (
	"sort"

	"tactile-atlas-gate/internal/domain"
)

const RuleVersion = "tactile-rules-2026.1"
const (
	RuleBraille      = "BRAILLE_CELLS"
	RuleDuplicate    = "DUPLICATE_LABEL"
	RuleSpacing      = "TACTILE_GAP"
	RuleConnectivity = "PATH_CONNECTIVITY"
	RuleLegend       = "LEGEND_REFERENCE"
	RuleBoundary     = "SHEET_BOUNDARY"
)

var AllRules = []string{RuleBoundary, RuleBraille, RuleDuplicate, RuleLegend, RuleConnectivity, RuleSpacing}

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) Validate(project domain.MapProject, proof domain.ProofRevision, selected []string) []domain.RuleResult {
	wanted := map[string]bool{}
	if len(selected) == 0 {
		selected = AllRules
	}
	for _, r := range selected {
		wanted[r] = true
	}
	checks := map[string]func(domain.MapProject, domain.ProofRevision) []domain.RuleResult{RuleBraille: checkBraille, RuleDuplicate: checkDuplicates, RuleSpacing: checkSpacing, RuleConnectivity: checkConnectivity, RuleLegend: checkLegend, RuleBoundary: checkBoundary}
	out := []domain.RuleResult{}
	for _, code := range AllRules {
		if wanted[code] {
			out = append(out, checks[code](project, proof)...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleCode != out[j].RuleCode {
			return out[i].RuleCode < out[j].RuleCode
		}
		if len(out[i].ElementRefs) == 0 {
			return true
		}
		if len(out[j].ElementRefs) == 0 {
			return false
		}
		return out[i].ElementRefs[0] < out[j].ElementRefs[0]
	})
	return out
}

func result(code string, passed bool, severity, message string, refs ...string) domain.RuleResult {
	sort.Strings(refs)
	return domain.RuleResult{RuleCode: code, Passed: passed, Severity: severity, ElementRefs: refs, Message: message}
}
