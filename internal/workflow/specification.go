package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"tactile-atlas-gate/internal/domain"
)

func (s *Service) PreflightSpecification(id, actor string, expected int64) (domain.SpecificationPreflight, error) {
	if actor == "" {
		return domain.SpecificationPreflight{}, fmt.Errorf("%w: actor 不能为空", domain.ErrInvalid)
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.SpecificationPreflight{}, err
	}
	if a.Project.Revision != expected {
		return domain.SpecificationPreflight{}, domain.ErrConflict
	}
	if a.Project.Status != domain.StatusDraft {
		return domain.SpecificationPreflight{}, domain.ErrInvalidState
	}
	preflight := domain.PreflightSpecification(a.Project, time.Now())
	a.SpecPreflight = &preflight
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "spec.preflight", Actor: actor, At: preflight.CheckedAt, Summary: map[bool]string{true: "规格冻结预检通过", false: "规格冻结预检发现冲突"}[preflight.Ready], Evidence: map[string]string{"preflight_digest": preflight.SummaryDigest, "spec_digest": preflight.SpecDigest}})
	err = s.repo.Save(a, expected, event("spec.preflight", actor, expected, preflight))
	return preflight, err
}

func (s *Service) FreezeWithPreflight(id, actor, request string, expected int64, preflightDigest string) (domain.Aggregate, error) {
	if err := validCommand(actor, request); err != nil {
		return domain.Aggregate{}, err
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	payload := struct {
		Actor           string
		Expected        int64
		PreflightDigest string
	}{actor, expected, preflightDigest}
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
	current := domain.PreflightSpecification(a.Project, time.Now())
	if a.SpecPreflight == nil || !a.SpecPreflight.Ready || a.SpecPreflight.Revision != expected || a.SpecPreflight.SpecDigest != current.SpecDigest || a.SpecPreflight.SummaryDigest != preflightDigest {
		return a, fmt.Errorf("%w: 规格预检摘要已过期或未通过", domain.ErrApprovalGate)
	}
	if err = a.Project.Transition(domain.StatusFrozen); err != nil {
		return a, err
	}
	a.Project.Revision++
	a.Timeline = append(a.Timeline, domain.TimelineEvent{Sequence: int64(len(a.Timeline) + 1), Type: "spec.frozen", Actor: actor, At: time.Now().UTC(), Summary: "制作规格已冻结", Evidence: map[string]string{"preflight_digest": a.SpecPreflight.SummaryDigest, "spec_digest": a.SpecPreflight.SpecDigest}})
	b, _ := json.Marshal(a)
	s.saveIdem(&a, request, payload, b)
	return a, s.repo.Save(a, expected, event("spec.frozen", actor, a.Project.Revision, map[string]string{"preflight_digest": a.SpecPreflight.SummaryDigest, "spec_digest": a.SpecPreflight.SpecDigest}))
}
