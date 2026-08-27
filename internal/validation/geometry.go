package validation

import (
	"fmt"
	"math"

	"tactile-atlas-gate/internal/domain"
)

type shape struct {
	id      string
	x, y, r float64
}

func shapes(p domain.ProofRevision) []shape {
	v := []shape{}
	for _, s := range p.TactileSymbols {
		v = append(v, shape{s.ID, s.X, s.Y, s.RadiusMM})
	}
	for _, l := range p.BrailleLabels {
		r := math.Hypot(l.WidthMM, l.HeightMM) / 2
		v = append(v, shape{l.ID, l.X + l.WidthMM/2, l.Y + l.HeightMM/2, r})
	}
	return v
}
func checkSpacing(project domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	v := shapes(p)
	out := []domain.RuleResult{}
	for i := 0; i < len(v); i++ {
		for j := i + 1; j < len(v); j++ {
			gap := math.Hypot(v[i].x-v[j].x, v[i].y-v[j].y) - v[i].r - v[j].r
			if gap < project.MinimumGapMM {
				out = append(out, result(RuleSpacing, false, "严重", fmt.Sprintf("元素间距 %.2f mm，小于 %.2f mm", gap, project.MinimumGapMM), v[i].id, v[j].id))
			}
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleSpacing, true, "信息", "触觉元素间距合格"))
	}
	return out
}
func checkBoundary(project domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	out := []domain.RuleResult{}
	for _, s := range p.TactileSymbols {
		if s.RadiusMM <= 0 || s.X-s.RadiusMM < 0 || s.Y-s.RadiusMM < 0 || s.X+s.RadiusMM > project.SheetWidthMM || s.Y+s.RadiusMM > project.SheetHeightMM {
			out = append(out, result(RuleBoundary, false, "严重", "触觉符号超出成品边界", s.ID))
		}
	}
	for _, l := range p.BrailleLabels {
		if l.WidthMM <= 0 || l.HeightMM <= 0 || l.X < 0 || l.Y < 0 || l.X+l.WidthMM > project.SheetWidthMM || l.Y+l.HeightMM > project.SheetHeightMM {
			out = append(out, result(RuleBoundary, false, "严重", "盲文标签超出成品边界", l.ID))
		}
	}
	for _, x := range p.Landmarks {
		if x.X < 0 || x.Y < 0 || x.X > project.SheetWidthMM || x.Y > project.SheetHeightMM {
			out = append(out, result(RuleBoundary, false, "严重", "导览点超出成品边界", x.ID))
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleBoundary, true, "信息", "全部元素位于成品边界内"))
	}
	return out
}
