package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"tactile-atlas-gate/internal/domain"
)

type FindingFilter struct {
	Source, Severity, Status, RuleCode, ElementRef string
	ProofSequence                                  int
}
type FindingList struct {
	Items         []domain.ReviewFinding `json:"items"`
	Count         int                    `json:"count"`
	BlockingCount int                    `json:"blocking_count"`
}

func (s *Service) FilterFindings(id string, filter FindingFilter) (FindingList, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return FindingList{}, err
	}
	proofSequence := map[string]int{}
	for _, p := range a.Proofs {
		proofSequence[p.ProofID] = p.Sequence
	}
	out := FindingList{Items: []domain.ReviewFinding{}}
	for _, f := range a.Findings {
		if filter.Source != "" && f.Source != filter.Source {
			continue
		}
		if filter.Severity != "" && f.Severity != filter.Severity {
			continue
		}
		if filter.Status != "" && string(f.Status) != filter.Status {
			continue
		}
		if filter.RuleCode != "" && f.RuleCode != filter.RuleCode {
			continue
		}
		if filter.ProofSequence > 0 && proofSequence[f.ProofID] != filter.ProofSequence {
			continue
		}
		if filter.ElementRef != "" && !contains(f.ElementRefs, filter.ElementRef) {
			continue
		}
		out.Items = append(out.Items, f)
		if f.Blocking() {
			out.BlockingCount++
		}
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].FindingID < out.Items[j].FindingID })
	out.Count = len(out.Items)
	return out, nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

type BatchFindingInput struct {
	FindingIDs []string          `json:"finding_ids"`
	Severity   string            `json:"severity,omitempty"`
	Status     string            `json:"status,omitempty"`
	Evidence   map[string]string `json:"evidence,omitempty"`
}

func (s *Service) BatchDecideFindings(id, actor, request string, expected int64, in BatchFindingInput) (domain.Aggregate, error) {
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
		Input    BatchFindingInput
	}{actor, expected, in}
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
	if len(in.FindingIDs) == 0 {
		return a, fmt.Errorf("%w: 至少选择一个问题", domain.ErrInvalid)
	}
	if in.Severity != "" && in.Severity != "严重" && in.Severity != "一般" && in.Severity != "提示" {
		return a, fmt.Errorf("%w: 问题严重级别非法", domain.ErrInvalid)
	}
	if in.Severity == "" && in.Status == "" {
		return a, fmt.Errorf("%w: 批量命令必须指定目标级别或状态", domain.ErrInvalid)
	}
	next := domain.FindingStatus(in.Status)
	if in.Status != "" && next != domain.FindingConfirmed && next != domain.FindingRejected && next != domain.FindingClosed {
		return a, fmt.Errorf("%w: 问题状态非法", domain.ErrInvalid)
	}
	index := map[string]int{}
	for i, f := range a.Findings {
		index[f.FindingID] = i
	}
	seen := map[string]bool{}
	failures := []string{}
	for _, fid := range in.FindingIDs {
		if seen[fid] {
			failures = append(failures, fid+": 重复选择")
			continue
		}
		seen[fid] = true
		i, ok := index[fid]
		if !ok {
			failures = append(failures, fid+": 不存在")
			continue
		}
		f := a.Findings[i]
		if next != "" && !allowedFindingTransition(f.Status, next) {
			failures = append(failures, fid+": 不允许从 "+string(f.Status)+" 转为 "+string(next))
		}
		if next == domain.FindingClosed {
			if strings.TrimSpace(in.Evidence[fid]) == "" {
				failures = append(failures, fid+": 缺少独立整改证据")
			} else if !findingMayClose(a, f) {
				failures = append(failures, fid+": 尚未通过对应复验门禁")
			}
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return a, fmt.Errorf("%w: 批量裁定失败: %s", domain.ErrApprovalGate, strings.Join(failures, "；"))
	}
	now := time.Now().UTC()
	details := []map[string]string{}
	for _, fid := range in.FindingIDs {
		i := index[fid]
		if in.Severity != "" {
			a.Findings[i].Severity = in.Severity
		}
		if next != "" {
			a.Findings[i].Status = next
			a.Findings[i].ResolutionEvidence = in.Evidence[fid]
			a.Findings[i].EvidenceReference = in.Evidence[fid]
			a.Findings[i].DecidedBy = actor
			a.Findings[i].DecidedAt = &now
		}
		details = append(details, map[string]string{"finding_id": fid, "severity": a.Findings[i].Severity, "status": string(a.Findings[i].Status), "evidence": in.Evidence[fid]})
	}
	if !a.HasBlockingFindings() {
		if r := a.LatestRun(); r != nil && r.AllPassed() && allRuleEvidencePassed(a) {
			a.Project.Status = domain.StatusApproval
		}
	}
	a.Project.Revision++
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "findings.batch_decided", Actor: actor, At: now, Summary: fmt.Sprintf("批量裁定 %d 个问题", len(in.FindingIDs)), Evidence: map[string]string{"request_id": request, "count": fmt.Sprint(len(in.FindingIDs))}})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	return a, s.repo.Save(a, expected, event("findings.batch_decided", actor, a.Project.Revision, details))
}
func allowedFindingTransition(from, to domain.FindingStatus) bool {
	if from == domain.FindingOpen {
		return to == domain.FindingConfirmed || to == domain.FindingRejected || to == domain.FindingClosed
	}
	if from == domain.FindingConfirmed {
		return to == domain.FindingRejected || to == domain.FindingClosed
	}
	return false
}
