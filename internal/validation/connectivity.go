package validation

import "tactile-atlas-gate/internal/domain"

func checkConnectivity(_ domain.MapProject, p domain.ProofRevision) []domain.RuleResult {
	if len(p.Landmarks) < 2 {
		return []domain.RuleResult{result(RuleConnectivity, true, "信息", "单导览点无需连通检查")}
	}
	known := map[string]bool{}
	adj := map[string][]string{}
	out := []domain.RuleResult{}
	for _, l := range p.Landmarks {
		known[l.ID] = true
	}
	for _, s := range p.PathSegments {
		if !known[s.FromLandmarkID] || !known[s.ToLandmarkID] || s.FromLandmarkID == s.ToLandmarkID {
			out = append(out, result(RuleConnectivity, false, "严重", "路径端点引用无效", s.ID))
			continue
		}
		adj[s.FromLandmarkID] = append(adj[s.FromLandmarkID], s.ToLandmarkID)
		adj[s.ToLandmarkID] = append(adj[s.ToLandmarkID], s.FromLandmarkID)
	}
	seen := map[string]bool{}
	q := []string{p.Landmarks[0].ID}
	for len(q) > 0 {
		x := q[0]
		q = q[1:]
		if seen[x] {
			continue
		}
		seen[x] = true
		q = append(q, adj[x]...)
	}
	for _, l := range p.Landmarks {
		if !seen[l.ID] {
			out = append(out, result(RuleConnectivity, false, "严重", "导览点不在连通路径中", l.ID))
		}
	}
	if len(out) == 0 {
		out = append(out, result(RuleConnectivity, true, "信息", "路径网络连通"))
	}
	return out
}
