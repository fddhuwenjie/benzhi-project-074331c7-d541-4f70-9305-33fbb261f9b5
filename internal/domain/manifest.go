package domain

import "time"

type ReleaseManifest struct {
	ManifestID         string            `json:"manifest_id"`
	ProjectID          string            `json:"project_id"`
	SpecDigest         string            `json:"spec_digest"`
	ProofDigest        string            `json:"proof_digest"`
	ValidationRunID    string            `json:"validation_run_id"`
	RuleRunIDs         map[string]string `json:"rule_run_ids"`
	ClosedFindingIDs   []string          `json:"closed_finding_ids"`
	FinalProofSequence int               `json:"final_proof_sequence"`
	ApprovedRevision   int64             `json:"approved_revision"`
	GateDigest         string            `json:"gate_digest"`
	GateResults        []GateItem        `json:"gate_results"`
	ApprovedBy         string            `json:"approved_by"`
	ApprovedAt         time.Time         `json:"approved_at"`
	CanonicalDigest    string            `json:"canonical_digest"`
}

type TimelineEvent struct {
	Sequence int64             `json:"sequence"`
	Type     string            `json:"type"`
	Actor    string            `json:"actor"`
	At       time.Time         `json:"at"`
	Summary  string            `json:"summary"`
	Evidence map[string]string `json:"evidence,omitempty"`
}

type IdempotencyRecord struct {
	Fingerprint string `json:"fingerprint"`
	Response    []byte `json:"response"`
}
type Aggregate struct {
	Project       MapProject                   `json:"project"`
	Proofs        []ProofRevision              `json:"proofs"`
	Runs          []ValidationRun              `json:"validation_runs"`
	Findings      []ReviewFinding              `json:"findings"`
	SpecPreflight *SpecificationPreflight      `json:"spec_preflight,omitempty"`
	Manifest      *ReleaseManifest             `json:"manifest,omitempty"`
	Timeline      []TimelineEvent              `json:"timeline"`
	Idempotency   map[string]IdempotencyRecord `json:"idempotency"`
}

func (a Aggregate) LatestProof() *ProofRevision {
	if len(a.Proofs) == 0 {
		return nil
	}
	p := a.Proofs[len(a.Proofs)-1]
	return &p
}
func (a Aggregate) LatestRun() *ValidationRun {
	if len(a.Runs) == 0 {
		return nil
	}
	r := a.Runs[len(a.Runs)-1]
	return &r
}
func (a Aggregate) HasBlockingFindings() bool {
	for _, f := range a.Findings {
		if f.Blocking() {
			return true
		}
	}
	return false
}
