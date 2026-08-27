package domain

import "time"

type RuleResult struct {
	RuleCode    string   `json:"rule_code"`
	Passed      bool     `json:"passed"`
	Severity    string   `json:"severity"`
	ElementRefs []string `json:"element_refs"`
	Message     string   `json:"message"`
}
type ValidationRun struct {
	RunID         string       `json:"run_id"`
	ProjectID     string       `json:"project_id"`
	ProofID       string       `json:"proof_id"`
	ProofSequence int          `json:"proof_sequence"`
	RuleVersion   string       `json:"rule_version"`
	Targeted      bool         `json:"targeted"`
	Rules         []string     `json:"rules"`
	ImpactDigest  string       `json:"impact_digest,omitempty"`
	Results       []RuleResult `json:"results"`
	CreatedAt     time.Time    `json:"created_at"`
}

func (r ValidationRun) AllPassed() bool {
	for _, x := range r.Results {
		if !x.Passed {
			return false
		}
	}
	return len(r.Results) > 0
}
