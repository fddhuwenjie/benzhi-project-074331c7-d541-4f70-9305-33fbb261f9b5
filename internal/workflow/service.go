package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
)

type Service struct {
	repo      repository.Repository
	engine    *validation.Engine
	locks     sync.Map
	gateMu    sync.Mutex
	gateCache map[string]domain.ApprovalGate
}

func New(repo repository.Repository, engine *validation.Engine) *Service {
	return &Service{repo: repo, engine: engine, gateCache: map[string]domain.ApprovalGate{}}
}
func (s *Service) lock(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}
func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func validCommand(actor, request string) error {
	if actor == "" || request == "" {
		return fmt.Errorf("%w: actor 和 request_id 不能为空", domain.ErrInvalid)
	}
	return nil
}
func (s *Service) idem(a *domain.Aggregate, id string, payload any) ([]byte, error) {
	if id == "" {
		return nil, nil
	}
	fp := fingerprint(payload)
	if rec, ok := a.Idempotency[id]; ok {
		if rec.Fingerprint != fp {
			return nil, domain.ErrIdempotency
		}
		return rec.Response, nil
	}
	return nil, nil
}
func (s *Service) saveIdem(a *domain.Aggregate, id string, payload any, response []byte) {
	if id != "" {
		if a.Idempotency == nil {
			a.Idempotency = map[string]domain.IdempotencyRecord{}
		}
		a.Idempotency[id] = domain.IdempotencyRecord{Fingerprint: fingerprint(payload), Response: response}
	}
}
func event(t, actor string, rev int64, p any) repository.Event {
	return repository.Event{Type: t, Actor: actor, At: time.Now().UTC().Format(time.RFC3339Nano), ProjectRevision: rev, Payload: p}
}

type CreateInput struct {
	ProjectID, Title, VenueZone   string
	Width, Height, Gap            float64
	Standard, Reviewer, RequestID string
}

func (s *Service) CreateProject(in CreateInput) (domain.Aggregate, error) {
	if err := validCommand(in.Reviewer, in.RequestID); err != nil {
		return domain.Aggregate{}, err
	}
	p, err := domain.NewProject(in.ProjectID, in.Title, in.VenueZone, in.Width, in.Height, in.Gap, in.Standard, in.Reviewer, time.Now())
	if err != nil {
		return domain.Aggregate{}, err
	}
	a := domain.Aggregate{Project: p, Idempotency: map[string]domain.IdempotencyRecord{}, Timeline: []domain.TimelineEvent{{Sequence: 1, Type: "project.created", Actor: in.Reviewer, At: time.Now().UTC(), Summary: "导览图项目已创建"}}}
	b, _ := json.Marshal(a)
	s.saveIdem(&a, in.RequestID, in, b)
	err = s.repo.Create(a, event("project.created", in.Reviewer, 0, in))
	if err == domain.ErrConflict {
		old, e := s.repo.Load(in.ProjectID)
		if e == nil {
			if rec, ok := old.Idempotency[in.RequestID]; ok && rec.Fingerprint == fingerprint(in) {
				return old, nil
			}
			return old, domain.ErrIdempotency
		}
	}
	return a, err
}
func (s *Service) Get(id string) (domain.Aggregate, error) { return s.repo.Load(id) }
func (s *Service) List() ([]domain.Aggregate, error)       { return s.repo.List() }
func (s *Service) Freeze(id, actor, request string, expected int64) (domain.Aggregate, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	if _, ok := a.Idempotency[request]; ok && a.SpecPreflight != nil {
		return s.FreezeWithPreflight(id, actor, request, expected, a.SpecPreflight.SummaryDigest)
	}
	if a.Project.Revision != expected {
		return a, domain.ErrConflict
	}
	if a.SpecPreflight == nil || a.SpecPreflight.Revision != expected {
		if _, err = s.PreflightSpecification(id, actor, expected); err != nil {
			return a, err
		}
		a, err = s.repo.Load(id)
		if err != nil {
			return a, err
		}
	}
	return s.FreezeWithPreflight(id, actor, request, expected, a.SpecPreflight.SummaryDigest)
}

func (s *Service) SubmitProof(id, actor, request string, expected int64, p domain.ProofRevision) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	payload := struct {
		Actor    string
		Expected int64
		Proof    domain.ProofRevision
	}{actor, expected, p}
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	if rec, ok := a.Idempotency[request]; ok {
		if rec.Fingerprint != fingerprint(payload) {
			return a, domain.ErrIdempotency
		}
		var old domain.Aggregate
		return old, json.Unmarshal(rec.Response, &old)
	}
	if a.Project.Revision != expected {
		return a, domain.ErrConflict
	}
	if a.Project.Status == domain.StatusPublished {
		return a, domain.ErrPublished
	}
	if a.Project.Status != domain.StatusFrozen && a.Project.Status != domain.StatusRemediation {
		return a, domain.ErrInvalidState
	}
	p.ProjectID = id
	p.Sequence = len(a.Proofs) + 1
	p.SubmittedBy = actor
	p.SubmittedAt = time.Now().UTC()
	p.Normalize()
	preflight := domain.PreflightProof(p)
	if !preflight.Ready {
		err = p.Validate()
		return a, err
	}
	for _, existing := range a.Proofs {
		if existing.ProofID == p.ProofID {
			return a, fmt.Errorf("%w: proof_id 已存在", domain.ErrInvalid)
		}
	}
	declared := map[string]bool{}
	for _, findingID := range p.DeclaredFindingIDs {
		if declared[findingID] {
			return a, fmt.Errorf("%w: declared_finding_ids 重复: %s", domain.ErrInvalid, findingID)
		}
		declared[findingID] = true
		found := false
		for _, f := range a.Findings {
			if f.FindingID == findingID {
				found = true
				if f.ProjectID != id || f.Status == domain.FindingRejected || f.Status == domain.FindingClosed {
					return a, fmt.Errorf("%w: 问题不可用于本次整改: %s", domain.ErrInvalid, findingID)
				}
			}
		}
		if !found {
			return a, fmt.Errorf("%w: 声明整改的问题不存在: %s", domain.ErrInvalid, findingID)
		}
	}
	if len(a.Proofs) > 0 {
		diff := domain.DiffProofs(a.Proofs[len(a.Proofs)-1], p)
		matrix := s.buildImpactMatrix(a, diff, p)
		if matrix.Blocked {
			return a, fmt.Errorf("%w: 版本差异包含悬空引用，不能提交或复验", domain.ErrInvalid)
		}
		p.ImpactDigest = matrix.ImpactDigest
		changed := diff.ChangedIDSet()
		for i := range a.Findings {
			if declared[a.Findings[i].FindingID] {
				a.Findings[i].RemediationProofID = p.ProofID
				a.Findings[i].ValidationRunID = ""
				a.Findings[i].EvidenceReference = ""
				a.Findings[i].CoverageStatus = domain.CoverageUncovered
				for _, ref := range a.Findings[i].ElementRefs {
					if changed[ref] {
						a.Findings[i].CoverageStatus = domain.CoveragePending
						break
					}
				}
				if a.Findings[i].RuleCode == "MANUAL" && len(diff.Changes) > 0 {
					a.Findings[i].CoverageStatus = domain.CoveragePending
				}
			}
		}
	}
	a.Proofs = append(a.Proofs, p)
	a.Project.Revision++
	a.Project.Status = domain.StatusChecking
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "proof.submitted", Actor: actor, At: time.Now().UTC(), Summary: fmt.Sprintf("校样 v%d 已提交", p.Sequence), Evidence: map[string]string{"preview_digest": preflight.PreviewDigest, "impact_digest": p.ImpactDigest}})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	err = s.repo.Save(a, expected, event("proof.submitted", actor, a.Project.Revision, p))
	return a, err
}

func (s *Service) Validate(id, actor, request string, expected int64, selected []string) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	payload := struct {
		Actor    string
		Expected int64
		Rules    []string
	}{actor, expected, selected}
	if rec, ok := a.Idempotency[request]; ok {
		if rec.Fingerprint != fingerprint(payload) {
			return a, domain.ErrIdempotency
		}
		var old domain.Aggregate
		return old, json.Unmarshal(rec.Response, &old)
	}
	if a.Project.Revision != expected {
		return a, domain.ErrConflict
	}
	if a.Project.Status != domain.StatusChecking {
		return a, domain.ErrInvalidState
	}
	p := a.LatestProof()
	if p == nil {
		return a, domain.ErrInvalid
	}
	if len(a.Proofs) == 1 {
		selected = append([]string(nil), validation.AllRules...)
	} else {
		d := domain.DiffProofs(a.Proofs[len(a.Proofs)-2], a.Proofs[len(a.Proofs)-1])
		matrix := s.buildImpactMatrix(a, d, *p)
		if matrix.Blocked {
			return a, fmt.Errorf("%w: 影响矩阵存在阻断性悬空引用", domain.ErrApprovalGate)
		}
		findingRules := []string{}
		declared := map[string]bool{}
		for _, fid := range p.DeclaredFindingIDs {
			declared[fid] = true
		}
		for _, f := range a.Findings {
			if declared[f.FindingID] {
				findingRules = append(findingRules, f.RuleCode)
			}
		}
		selected = validation.RequiredRules(d, findingRules, selected)
	}
	results := s.engine.Validate(a.Project, *p, selected)
	run := domain.ValidationRun{RunID: fmt.Sprintf("run-%d", time.Now().UnixNano()), ProjectID: id, ProofID: p.ProofID, ProofSequence: p.Sequence, RuleVersion: validation.RuleVersion, Targeted: len(a.Proofs) > 1, Rules: selected, ImpactDigest: p.ImpactDigest, Results: results, CreatedAt: time.Now().UTC()}
	a.Runs = append(a.Runs, run)
	for i := range a.Findings {
		if a.Findings[i].RemediationProofID != p.ProofID || a.Findings[i].CoverageStatus == domain.CoverageUncovered {
			continue
		}
		passed := a.Findings[i].RuleCode == "MANUAL" && run.AllPassed()
		for _, r := range results {
			if r.RuleCode == a.Findings[i].RuleCode {
				passed = r.Passed
				break
			}
		}
		a.Findings[i].ValidationRunID = run.RunID
		if passed {
			a.Findings[i].CoverageStatus = domain.CoverageClosable
		} else {
			a.Findings[i].CoverageStatus = domain.CoverageFailed
		}
	}
	if run.AllPassed() && allRuleEvidencePassed(a) {
		if a.HasBlockingFindings() {
			a.Project.Status = domain.StatusRemediation
		} else {
			a.Project.Status = domain.StatusApproval
		}
	} else {
		a.Project.Status = domain.StatusRemediation
		for _, r := range results {
			if !r.Passed {
				a.Findings = append(a.Findings, domain.ReviewFinding{FindingID: fmt.Sprintf("finding-%s-%d", r.RuleCode, time.Now().UnixNano()), ProjectID: id, ProofID: p.ProofID, Source: "rule", Severity: r.Severity, RuleCode: r.RuleCode, ElementRefs: r.ElementRefs, Description: r.Message, Status: domain.FindingOpen})
			}
		}
	}
	a.Project.Revision++
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "validation.completed", Actor: actor, At: time.Now().UTC(), Summary: fmt.Sprintf("规则检查 %s", map[bool]string{true: "通过", false: "发现问题"}[run.AllPassed()])})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	err = s.repo.Save(a, expected, event("validation.completed", actor, a.Project.Revision, run))
	return a, err
}

func (s *Service) DecideFinding(id, findingID, actor, request, status, evidence string, expected int64) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	payload := struct {
		FindingID, Actor, Status, Evidence string
		Expected                           int64
	}{findingID, actor, status, evidence, expected}
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	if rec, ok := a.Idempotency[request]; ok {
		if rec.Fingerprint != fingerprint(payload) {
			return a, domain.ErrIdempotency
		}
		var old domain.Aggregate
		return old, json.Unmarshal(rec.Response, &old)
	}
	if a.Project.Revision != expected {
		return a, domain.ErrConflict
	}
	if a.Project.Status == domain.StatusPublished {
		return a, domain.ErrPublished
	}
	if actor != a.Project.ReviewerID {
		return a, domain.ErrReviewer
	}
	next := domain.FindingStatus(status)
	if next != domain.FindingConfirmed && next != domain.FindingRejected && next != domain.FindingClosed {
		return a, fmt.Errorf("%w: 问题状态非法", domain.ErrInvalid)
	}
	if next == domain.FindingClosed && evidence == "" {
		return a, fmt.Errorf("%w: 关闭问题必须提供整改证据", domain.ErrInvalid)
	}
	for i := range a.Findings {
		if a.Findings[i].FindingID == findingID {
			if next == domain.FindingClosed && !findingMayClose(a, a.Findings[i]) {
				return a, fmt.Errorf("%w: 问题尚未完成对应规则复验", domain.ErrApprovalGate)
			}
			a.Findings[i].Status = next
			a.Findings[i].ResolutionEvidence = evidence
			a.Findings[i].EvidenceReference = evidence
			a.Findings[i].DecidedBy = actor
			t := time.Now().UTC()
			a.Findings[i].DecidedAt = &t
			a.Project.Revision++
			if !a.HasBlockingFindings() {
				if r := a.LatestRun(); r != nil && r.AllPassed() && allRuleEvidencePassed(a) {
					a.Project.Status = domain.StatusApproval
				}
			}
			a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "finding.decided", Actor: actor, At: t, Summary: "问题已裁定为 " + status})
			b, _ := json.Marshal(a)
			s.saveIdem(&a, request, payload, b)
			return a, s.repo.Save(a, expected, event("finding.decided", actor, a.Project.Revision, a.Findings[i]))
		}
	}
	return a, domain.ErrNotFound
}

func (s *Service) Approve(id, actor, request string, expected int64) (domain.Aggregate, error) {
	gate, err := s.ApprovalGate(id, actor)
	if err != nil {
		return domain.Aggregate{}, err
	}
	return s.ApproveWithGate(id, actor, request, expected, gate.SummaryDigest)
}

func (s *Service) AddFinding(id, actor, request, severity, description string, refs []string, expected int64) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	a, err := s.repo.Load(id)
	if err != nil {
		return a, err
	}
	payload := struct {
		Actor, Severity, Description string
		Refs                         []string
		Expected                     int64
	}{actor, severity, description, refs, expected}
	if rec, ok := a.Idempotency[request]; ok {
		if rec.Fingerprint != fingerprint(payload) {
			return a, domain.ErrIdempotency
		}
		var old domain.Aggregate
		return old, json.Unmarshal(rec.Response, &old)
	}
	if a.Project.Revision != expected {
		return a, domain.ErrConflict
	}
	if a.Project.Status == domain.StatusPublished {
		return a, domain.ErrPublished
	}
	if actor != a.Project.ReviewerID {
		return a, domain.ErrReviewer
	}
	p := a.LatestProof()
	if p == nil || strings.TrimSpace(description) == "" {
		return a, domain.ErrInvalid
	}
	if severity != "严重" && severity != "一般" && severity != "提示" {
		return a, fmt.Errorf("%w: 问题严重级别非法", domain.ErrInvalid)
	}
	f := domain.ReviewFinding{FindingID: fmt.Sprintf("manual-%d", time.Now().UnixNano()), ProjectID: id, ProofID: p.ProofID, Source: "manual", Severity: severity, RuleCode: "MANUAL", ElementRefs: refs, Description: description, Status: domain.FindingOpen}
	a.Findings = append(a.Findings, f)
	a.Project.Status = domain.StatusRemediation
	a.Project.Revision++
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "finding.created", Actor: actor, At: time.Now().UTC(), Summary: "复核员新增人工问题"})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	return a, s.repo.Save(a, expected, event("finding.created", actor, a.Project.Revision, f))
}

func (s *Service) Diff(id string, from, to int) (domain.ProofDiff, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.ProofDiff{}, err
	}
	if from < 1 || to < 1 || from > len(a.Proofs) || to > len(a.Proofs) {
		return domain.ProofDiff{}, domain.ErrNotFound
	}
	return domain.DiffProofs(a.Proofs[from-1], a.Proofs[to-1]), nil
}
func (s *Service) VerifyManifest(id string) (bool, *domain.ReleaseManifest, error) {
	report, err := s.VerifyManifestDetailed(id)
	return report.Valid, report.Manifest, err
}
