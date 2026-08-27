package domain

import "time"

type GateItem struct {
	Code         string `json:"code"`
	Label        string `json:"label"`
	Passed       bool   `json:"passed"`
	Blocking     bool   `json:"blocking"`
	Reason       string `json:"reason,omitempty"`
	SourceRunID  string `json:"source_run_id,omitempty"`
	RuleVersion  string `json:"rule_version,omitempty"`
	EvidenceMode string `json:"evidence_mode,omitempty"`
}
type ApprovalGate struct {
	Revision         int64      `json:"revision"`
	Ready            bool       `json:"ready"`
	Items            []GateItem `json:"items"`
	FinalProofDigest string     `json:"final_proof_digest,omitempty"`
	SummaryDigest    string     `json:"summary_digest"`
	GeneratedAt      time.Time  `json:"generated_at"`
}
type ManifestVerificationItem struct {
	Type           string   `json:"type"`
	Passed         bool     `json:"passed"`
	ExpectedDigest string   `json:"expected_digest,omitempty"`
	ActualDigest   string   `json:"actual_digest,omitempty"`
	ReferenceIDs   []string `json:"reference_ids,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}
type ManifestVerification struct {
	Valid           bool                       `json:"valid"`
	CheckedAt       time.Time                  `json:"checked_at"`
	ProjectRevision int64                      `json:"project_revision"`
	Items           []ManifestVerificationItem `json:"items"`
	Manifest        *ReleaseManifest           `json:"manifest,omitempty"`
}
