package validation

import (
	"sort"
	"tactile-atlas-gate/internal/domain"
)

func AffectedRules(d domain.ProofDiff) []string {
	set := map[string]bool{RuleBoundary: true, RuleConnectivity: true}
	for _, s := range d.ChangedSections {
		switch s {
		case "braille_labels":
			set[RuleBraille] = true
			set[RuleDuplicate] = true
			set[RuleSpacing] = true
		case "tactile_symbols":
			set[RuleSpacing] = true
			set[RuleLegend] = true
		case "legend_entries":
			set[RuleLegend] = true
		case "path_segments", "landmarks":
			set[RuleConnectivity] = true
		}
	}
	out := []string{}
	for _, r := range AllRules {
		if set[r] {
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

func RulesForChange(change domain.FieldChange) []string {
	set := map[string]bool{RuleBoundary: true, RuleConnectivity: true}
	switch change.ItemType {
	case "braille_labels":
		set[RuleBraille], set[RuleDuplicate], set[RuleSpacing] = true, true, true
	case "tactile_symbols":
		set[RuleSpacing], set[RuleLegend] = true, true
	case "legend_entries":
		set[RuleLegend] = true
	case "path_segments", "landmarks":
		set[RuleConnectivity] = true
	}
	out := []string{}
	for _, code := range AllRules {
		if set[code] {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}

func RequiredRules(d domain.ProofDiff, findingRules, requested []string) []string {
	set := map[string]bool{}
	for _, code := range AffectedRules(d) {
		set[code] = true
	}
	for _, code := range findingRules {
		if code != "" && code != "MANUAL" {
			set[code] = true
		}
	}
	for _, code := range requested {
		for _, known := range AllRules {
			if code == known {
				set[code] = true
			}
		}
	}
	out := []string{}
	for _, code := range AllRules {
		if set[code] {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}
