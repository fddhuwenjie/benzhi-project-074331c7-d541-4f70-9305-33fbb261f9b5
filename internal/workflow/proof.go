package workflow

import (
	"sort"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/validation"
)

func (s *Service) PreflightProof(id, actor string, expected int64, p domain.ProofRevision) (domain.ProofPreflight, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.ProofPreflight{}, err
	}
	if a.Project.Revision != expected {
		return domain.ProofPreflight{}, domain.ErrConflict
	}
	if a.Project.Status != domain.StatusFrozen && a.Project.Status != domain.StatusRemediation {
		return domain.ProofPreflight{}, domain.ErrInvalidState
	}
	p.ProjectID = id
	p.Sequence = len(a.Proofs) + 1
	p.SubmittedBy = actor
	p.SubmittedAt = time.Time{}
	result := domain.PreflightProof(p)
	if len(a.Proofs) > 0 {
		d := domain.DiffProofs(a.Proofs[len(a.Proofs)-1], result.Preview)
		matrix := s.buildImpactMatrix(a, d, result.Preview)
		result.Impact = &matrix
		result.Preview.ImpactDigest = matrix.ImpactDigest
		if matrix.Blocked {
			result.Ready = false
			result.Issues = append(result.Issues, domain.FieldIssue{Section: "impact", Field: "references", Reason: "版本差异包含悬空引用，不能发起复验"})
		}
	}
	return result, nil
}

func (s *Service) buildImpactMatrix(a domain.Aggregate, d domain.ProofDiff, target domain.ProofRevision) domain.ProofDiff {
	openByRef := map[string][]string{}
	for _, f := range a.Findings {
		if !f.Blocking() {
			continue
		}
		for _, ref := range f.ElementRefs {
			openByRef[ref] = append(openByRef[ref], f.FindingID)
		}
	}
	issues := target.ValidationIssues()
	blocking := map[string]string{}
	for _, issue := range issues {
		if issue.Field == "symbol_id" || issue.Field == "label_id" || issue.Field == "from_landmark_id" || issue.Field == "to_landmark_id" || issue.Field == "legend_key" {
			blocking[issue.ItemID] = issue.Reason
		}
	}
	rows := make([]domain.ImpactRow, 0, len(d.Changes))
	for _, change := range d.Changes {
		ids := append([]string(nil), openByRef[change.ItemID]...)
		sort.Strings(ids)
		reason := blocking[change.ItemID]
		row := domain.ImpactRow{FieldChange: change, AffectedRules: validation.RulesForChange(change), FindingIDs: ids, GlobalBoundary: true, GlobalConnectivity: true, Blocking: reason != "", BlockingReason: reason}
		if row.Blocking {
			d.Blocked = true
		}
		rows = append(rows, row)
	}
	d.ImpactRows = rows
	summary := struct {
		From, To int
		Rows     []domain.ImpactRow
	}{d.FromSequence, d.ToSequence, rows}
	d.ImpactDigest, _ = domain.Digest(summary)
	return d
}

func (s *Service) ImpactMatrix(id string, from, to int) (domain.ProofDiff, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.ProofDiff{}, err
	}
	if from < 1 || to < 1 || from > len(a.Proofs) || to > len(a.Proofs) {
		return domain.ProofDiff{}, domain.ErrNotFound
	}
	d := domain.DiffProofs(a.Proofs[from-1], a.Proofs[to-1])
	if err = d.ValidateAdjacent(); err != nil {
		return d, err
	}
	return s.buildImpactMatrix(a, d, a.Proofs[to-1]), nil
}
