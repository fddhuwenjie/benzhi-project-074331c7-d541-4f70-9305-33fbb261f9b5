package validation

import "tactile-atlas-gate/internal/domain"

func checkLegend(_ domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	legend := map[string]bool{}
	out := []domain.RuleResult{}
	for _, e := range p.LegendEntries {
		if e.Key == "" || e.Meaning == "" {
			out = append(out, result(RuleLegend, false, "一般", "图例键或含义为空", "legend:"+e.Key))
		}
		legend[e.Key] = true
	}
	for _, s := range p.TactileSymbols {
		if !legend[s.LegendKey] {
			out = append(out, result(RuleLegend, false, "严重", "触觉符号未引用有效图例", s.ID))
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleLegend, true, "信息", "全部图例引用有效"))
	}
	return out
}
