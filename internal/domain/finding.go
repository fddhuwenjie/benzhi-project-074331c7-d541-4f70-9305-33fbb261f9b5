package domain

import "time"

type FindingStatus string

const (
	FindingOpen      FindingStatus = "未确认"
	FindingConfirmed FindingStatus = "已确认"
	FindingRejected  FindingStatus = "已驳回"
	FindingClosed    FindingStatus = "已关闭"
)

type ReviewFinding struct {
	FindingID          string         `json:"finding_id"`
	ProjectID          string         `json:"project_id"`
	ProofID            string         `json:"proof_id"`
	Source             string         `json:"source"`
	Severity           string         `json:"severity"`
	RuleCode           string         `json:"rule_code"`
	ElementRefs        []string       `json:"element_refs"`
	Description        string         `json:"description"`
	Status             FindingStatus  `json:"status"`
	ResolutionEvidence string         `json:"resolution_evidence,omitempty"`
	DecidedBy          string         `json:"decided_by,omitempty"`
	DecidedAt          *time.Time     `json:"decided_at,omitempty"`
	CoverageStatus     CoverageStatus `json:"coverage_status,omitempty"`
	RemediationProofID string         `json:"remediation_proof_id,omitempty"`
	ValidationRunID    string         `json:"validation_run_id,omitempty"`
	EvidenceReference  string         `json:"evidence_reference,omitempty"`
}

type CoverageStatus string

const (
	CoverageUncovered CoverageStatus = "未覆盖"
	CoveragePending   CoverageStatus = "已修改待复验"
	CoverageFailed    CoverageStatus = "复验失败"
	CoverageClosable  CoverageStatus = "可关闭"
)

func (f ReviewFinding) Blocking() bool {
	return f.Status == FindingOpen || f.Status == FindingConfirmed
}
