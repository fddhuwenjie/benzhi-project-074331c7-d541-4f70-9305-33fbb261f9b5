package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/validation"
)

type ruleEvidence struct {
	passed               bool
	runID, version, mode string
}

func approvalRuleEvidence(a domain.Aggregate) map[string]ruleEvidence {
	latest := a.LatestProof()
	out := map[string]ruleEvidence{}
	for _, run := range a.Runs {
		for _, result := range run.Results {
			mode := "全量检查"
			if run.Targeted {
				mode = "定向复验"
			}
			if latest != nil && run.ProofID != latest.ProofID {
				mode = "沿用复验结论"
			}
			out[result.RuleCode] = ruleEvidence{result.Passed, run.RunID, run.RuleVersion, mode}
		}
	}
	return out
}

func (s *Service) ApprovalGate(id, actor string) (domain.ApprovalGate, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.ApprovalGate{}, err
	}
	gate := domain.ApprovalGate{Revision: a.Project.Revision, GeneratedAt: time.Now().UTC()}
	items := []domain.GateItem{}
	items = append(items, domain.GateItem{Code: "PROJECT_STATUS", Label: "项目处于待批准状态", Passed: a.Project.Status == domain.StatusApproval, Blocking: true, Reason: map[bool]string{false: "项目尚未进入待批准状态"}[a.Project.Status == domain.StatusApproval]})
	p := a.LatestProof()
	independent := p != nil && actor == a.Project.ReviewerID && p.SubmittedBy != actor
	reason := ""
	if actor != a.Project.ReviewerID {
		reason = "批准人不是指定复核员"
	} else if p == nil {
		reason = "缺少最终校样"
	} else if p.SubmittedBy == actor {
		reason = "批准人与最终校样提交者必须相互独立"
	}
	items = append(items, domain.GateItem{Code: "REVIEWER_INDEPENDENCE", Label: "提交者与批准人相互独立", Passed: independent, Blocking: true, Reason: reason})
	evidence := approvalRuleEvidence(a)
	for _, code := range validation.AllRules {
		ev, ok := evidence[code]
		passed := ok && ev.passed
		reason := ""
		if !ok {
			reason = "缺少规则证据"
		} else if !ev.passed {
			reason = "最近规则证据未通过"
		}
		items = append(items, domain.GateItem{Code: "RULE_" + code, Label: "规则证据 " + code, Passed: passed, Blocking: true, Reason: reason, SourceRunID: ev.runID, RuleVersion: ev.version, EvidenceMode: ev.mode})
	}
	open := []string{}
	for _, f := range a.Findings {
		if f.Blocking() {
			open = append(open, f.FindingID)
		}
	}
	sort.Strings(open)
	items = append(items, domain.GateItem{Code: "OPEN_FINDINGS", Label: "所有问题均已关闭或驳回", Passed: len(open) == 0, Blocking: true, Reason: map[bool]string{false: "仍有未关闭问题: " + fmt.Sprint(open)}[len(open) == 0]})
	proofDigest := ""
	if p != nil {
		proofDigest, _ = domain.Digest(p)
	}
	items = append(items, domain.GateItem{Code: "FINAL_PROOF", Label: "最终校样摘要可用", Passed: p != nil && proofDigest != "", Blocking: true, Reason: map[bool]string{false: "缺少可摘要的最终校样"}[p != nil && proofDigest != ""]})
	gate.Items = items
	gate.FinalProofDigest = proofDigest
	gate.Ready = true
	for _, item := range items {
		if item.Blocking && !item.Passed {
			gate.Ready = false
		}
	}
	summary := struct {
		Revision         int64             `json:"revision"`
		Ready            bool              `json:"ready"`
		Items            []domain.GateItem `json:"items"`
		FinalProofDigest string            `json:"final_proof_digest"`
	}{gate.Revision, gate.Ready, gate.Items, gate.FinalProofDigest}
	gate.SummaryDigest, _ = domain.Digest(summary)
	return gate, nil
}

func (s *Service) ApproveWithGate(id, actor, request string, expected int64, gateDigest string) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	defer delete(s.locks, id)
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	payload := struct {
		Actor      string
		Expected   int64
		GateDigest string
	}{actor, expected, gateDigest}
	if rec, ok := a.Idempotency[request]; ok {
		if rec.Fingerprint != fingerprint(payload) {
			return a, domain.ErrIdempotency
		}
		var old domain.Aggregate
		return old, json.Unmarshal(rec.Response, &old)
	}
	if a.Project.Revision != expected {
		return a, fmt.Errorf("%w: 批准门禁摘要已过期，项目 revision 已变化", domain.ErrConflict)
	}
	if actor != a.Project.ReviewerID {
		return a, domain.ErrReviewer
	}
	gate, err := s.approvalGateFromAggregate(a, actor)
	if err != nil {
		return a, err
	}
	if gate.SummaryDigest != gateDigest || !gate.Ready {
		return a, fmt.Errorf("%w: 门禁摘要已过期或仍有阻断项", domain.ErrApprovalGate)
	}
	p := a.LatestProof()
	r := a.LatestRun()
	if p == nil || r == nil {
		return a, domain.ErrApprovalGate
	}
	specDigest, _ := domain.SpecificationDigest(a.Project)
	closed := []string{}
	for _, f := range a.Findings {
		if f.Status == domain.FindingClosed {
			closed = append(closed, f.FindingID)
		}
	}
	sort.Strings(closed)
	ruleRuns := map[string]string{}
	for _, item := range gate.Items {
		if len(item.Code) > 5 && item.Code[:5] == "RULE_" {
			ruleRuns[item.Code[5:]] = item.SourceRunID
		}
	}
	manifest := domain.ReleaseManifest{ManifestID: fmt.Sprintf("manifest-%d", time.Now().UnixNano()), ProjectID: id, SpecDigest: specDigest, ProofDigest: gate.FinalProofDigest, ValidationRunID: r.RunID, RuleRunIDs: ruleRuns, ClosedFindingIDs: closed, FinalProofSequence: p.Sequence, ApprovedRevision: expected, GateDigest: gate.SummaryDigest, GateResults: gate.Items, ApprovedBy: actor, ApprovedAt: time.Now().UTC()}
	manifest.CanonicalDigest, _ = domain.ManifestDigest(manifest)
	a.Manifest = &manifest
	a.Project.Status = domain.StatusPublished
	a.Project.Revision++
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "project.published", Actor: actor, At: manifest.ApprovedAt, Summary: "母版发布清单已冻结", Evidence: map[string]string{"gate_digest": gate.SummaryDigest, "manifest_digest": manifest.CanonicalDigest}})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	return a, s.repo.Save(a, expected, event("project.published", actor, a.Project.Revision, manifest))
}

func (s *Service) approvalGateFromAggregate(a domain.Aggregate, actor string) (domain.ApprovalGate, error) {
	// 与公开计算共用同一聚合快照，避免在项目锁内再次读取。
	gate := domain.ApprovalGate{Revision: a.Project.Revision, GeneratedAt: time.Now().UTC()}
	items := []domain.GateItem{}
	statusOK := a.Project.Status == domain.StatusApproval
	items = append(items, domain.GateItem{Code: "PROJECT_STATUS", Label: "项目处于待批准状态", Passed: statusOK, Blocking: true, Reason: map[bool]string{false: "项目尚未进入待批准状态"}[statusOK]})
	p := a.LatestProof()
	independent := p != nil && actor == a.Project.ReviewerID && p.SubmittedBy != actor
	independenceReason := ""
	if actor != a.Project.ReviewerID {
		independenceReason = "批准人不是指定复核员"
	} else if p == nil {
		independenceReason = "缺少最终校样"
	} else if p.SubmittedBy == actor {
		independenceReason = "批准人与最终校样提交者必须相互独立"
	}
	items = append(items, domain.GateItem{Code: "REVIEWER_INDEPENDENCE", Label: "提交者与批准人相互独立", Passed: independent, Blocking: true, Reason: independenceReason})
	evidence := approvalRuleEvidence(a)
	for _, code := range validation.AllRules {
		ev, ok := evidence[code]
		passed := ok && ev.passed
		reason := ""
		if !ok {
			reason = "缺少规则证据"
		} else if !ev.passed {
			reason = "最近规则证据未通过"
		}
		items = append(items, domain.GateItem{Code: "RULE_" + code, Label: "规则证据 " + code, Passed: passed, Blocking: true, Reason: reason, SourceRunID: ev.runID, RuleVersion: ev.version, EvidenceMode: ev.mode})
	}
	open := []string{}
	for _, f := range a.Findings {
		if f.Blocking() {
			open = append(open, f.FindingID)
		}
	}
	sort.Strings(open)
	items = append(items, domain.GateItem{Code: "OPEN_FINDINGS", Label: "所有问题均已关闭或驳回", Passed: len(open) == 0, Blocking: true, Reason: map[bool]string{false: "仍有未关闭问题: " + fmt.Sprint(open)}[len(open) == 0]})
	proofDigest := ""
	if p != nil {
		proofDigest, _ = domain.Digest(p)
	}
	proofOK := p != nil && proofDigest != ""
	items = append(items, domain.GateItem{Code: "FINAL_PROOF", Label: "最终校样摘要可用", Passed: proofOK, Blocking: true, Reason: map[bool]string{false: "缺少可摘要的最终校样"}[proofOK]})
	gate.Items = items
	gate.FinalProofDigest = proofDigest
	gate.Ready = true
	for _, item := range items {
		if item.Blocking && !item.Passed {
			gate.Ready = false
		}
	}
	summary := struct {
		Revision         int64             `json:"revision"`
		Ready            bool              `json:"ready"`
		Items            []domain.GateItem `json:"items"`
		FinalProofDigest string            `json:"final_proof_digest"`
	}{gate.Revision, gate.Ready, gate.Items, gate.FinalProofDigest}
	gate.SummaryDigest, _ = domain.Digest(summary)
	return gate, nil
}

func (s *Service) VerifyManifestDetailed(id string) (domain.ManifestVerification, error) {
	report := domain.ManifestVerification{CheckedAt: time.Now().UTC()}
	a, err := s.repo.Load(id)
	if err != nil {
		report.Items = []domain.ManifestVerificationItem{{Type: "不可变证据或事件链", Passed: false, Reason: err.Error()}}
		return report, nil
	}
	report.ProjectRevision = a.Project.Revision
	report.Manifest = a.Manifest
	if a.Manifest == nil {
		return report, domain.ErrNotFound
	}
	m := a.Manifest
	spec, _ := domain.SpecificationDigest(a.Project)
	report.Items = append(report.Items, domain.ManifestVerificationItem{Type: "规格摘要", Passed: spec == m.SpecDigest, ExpectedDigest: m.SpecDigest, ActualDigest: spec})
	proof := a.LatestProof()
	proofDigest := ""
	proofID := ""
	proofOK := proof != nil && proof.ProjectID == id && proof.Sequence == m.FinalProofSequence
	if proof != nil {
		proofID = proof.ProofID
		proofDigest, _ = domain.Digest(proof)
	}
	proofOK = proofOK && proofDigest == m.ProofDigest
	proofReason := ""
	if !proofOK {
		proofReason = "最终校样摘要、归属或序号不一致"
	}
	report.Items = append(report.Items, domain.ManifestVerificationItem{Type: "最终校样摘要", Passed: proofOK, ExpectedDigest: m.ProofDigest, ActualDigest: proofDigest, ReferenceIDs: []string{proofID}, Reason: proofReason})
	runs := map[string]domain.ValidationRun{}
	for _, run := range a.Runs {
		runs[run.RunID] = run
	}
	run, runOK := runs[m.ValidationRunID]
	runOK = runOK && run.ProjectID == id && proof != nil && run.ProofID == proof.ProofID && a.Project.Revision == m.ApprovedRevision+1 && len(m.RuleRunIDs) == len(validation.AllRules)
	refs := []string{}
	for _, code := range validation.AllRules {
		runID, present := m.RuleRunIDs[code]
		refs = append(refs, code+":"+runID)
		ev, ok := runs[runID]
		passed := false
		for _, result := range ev.Results {
			if result.RuleCode == code && result.Passed {
				passed = true
			}
		}
		if !present || !ok || ev.ProjectID != id || !passed {
			runOK = false
		}
	}
	sort.Strings(refs)
	report.Items = append(report.Items, domain.ManifestVerificationItem{Type: "批准运行引用", Passed: runOK, ReferenceIDs: refs, Reason: map[bool]string{false: "检查运行缺失、归属错误或未对应最终校样"}[runOK]})
	actualClosed := []string{}
	for _, f := range a.Findings {
		if f.Status == domain.FindingClosed {
			actualClosed = append(actualClosed, f.FindingID)
		}
	}
	sort.Strings(actualClosed)
	expectedClosed := append([]string{}, m.ClosedFindingIDs...)
	sort.Strings(expectedClosed)
	closedOK := fingerprint(actualClosed) == fingerprint(expectedClosed)
	report.Items = append(report.Items, domain.ManifestVerificationItem{Type: "关闭问题集合", Passed: closedOK, ExpectedDigest: fingerprint(expectedClosed), ActualDigest: fingerprint(actualClosed), ReferenceIDs: actualClosed, Reason: map[bool]string{false: "关闭问题集合存在缺失、替换或额外项"}[closedOK]})
	canonical, _ := domain.ManifestDigest(*m)
	canonicalOK := domain.VerifyManifest(*m)
	report.Items = append(report.Items, domain.ManifestVerificationItem{Type: "清单规范摘要", Passed: canonicalOK, ExpectedDigest: m.CanonicalDigest, ActualDigest: canonical, Reason: map[bool]string{false: "清单规范摘要不一致"}[canonicalOK]})
	report.Valid = true
	for _, item := range report.Items {
		if !item.Passed {
			report.Valid = false
		}
	}
	return report, nil
}
