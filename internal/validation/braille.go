package validation

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"tactile-atlas-gate/internal/domain"
)

func checkBraille(_ domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	out := []domain.RuleResult{}
	for _, l := range p.BrailleLabels {
		valid := l.Cells != ""
		for _, r := range l.Cells {
			if r < 0x2800 || r > 0x28ff {
				valid = false
				break
			}
		}
		if !valid {
			out = append(out, result(RuleBraille, false, "严重", fmt.Sprintf("盲文标签 %s 含非法单元", l.ID), l.ID))
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleBraille, true, "信息", "全部盲文单元合法"))
	}
	return out
}
func checkDuplicates(_ domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	by := map[string][]string{}
	for _, l := range p.BrailleLabels {
		k := strings.ToLower(strings.TrimSpace(l.Text))
		by[k] = append(by[k], l.ID)
	}
	out := []domain.RuleResult{}
	for text, ids := range by {
		if text != "" && len(ids) > 1 {
			out = append(out, result(RuleDuplicate, false, "一般", "盲文标签文字重复: "+text, ids...))
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleDuplicate, true, "信息", "盲文标签无重复"))
	}
	return out
}
func BrailleCellCount(s string) int { return utf8.RuneCountInString(s) }
