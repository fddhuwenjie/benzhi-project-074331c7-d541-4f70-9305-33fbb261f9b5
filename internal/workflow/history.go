package workflow

import (
	"sort"
	"time"

	"tactile-atlas-gate/internal/domain"
)

type ValidationRunFilter struct {
	ProofSequence int
	RuleVersion   string
	Targeted      *bool
	From, To      time.Time
}

func (s *Service) ValidationHistory(id string, filter ValidationRunFilter) ([]domain.ValidationRun, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return nil, err
	}
	out := []domain.ValidationRun{}
	for _, run := range a.Runs {
		if filter.ProofSequence > 0 && run.ProofSequence != filter.ProofSequence {
			continue
		}
		if filter.RuleVersion != "" && run.RuleVersion != filter.RuleVersion {
			continue
		}
		if filter.Targeted != nil && run.Targeted != *filter.Targeted {
			continue
		}
		if !filter.From.IsZero() && run.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && run.CreatedAt.After(filter.To) {
			continue
		}
		sort.SliceStable(run.Results, func(i, j int) bool {
			if run.Results[i].RuleCode != run.Results[j].RuleCode {
				return run.Results[i].RuleCode < run.Results[j].RuleCode
			}
			return joinRefs(run.Results[i].ElementRefs) < joinRefs(run.Results[j].ElementRefs)
		})
		out = append(out, run)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func joinRefs(v []string) string {
	copyV := append([]string(nil), v...)
	sort.Strings(copyV)
	out := ""
	for _, x := range copyV {
		out += "\x00" + x
	}
	return out
}
func (s *Service) CompareValidation(id, baselineID, targetID string) (domain.ValidationComparison, error) {
	a, err := s.repo.Load(id)
	if err != nil {
		return domain.ValidationComparison{}, err
	}
	var base, target *domain.ValidationRun
	for i := range a.Runs {
		if a.Runs[i].RunID == baselineID {
			base = &a.Runs[i]
		}
		if a.Runs[i].RunID == targetID {
			target = &a.Runs[i]
		}
	}
	if base == nil || target == nil {
		return domain.ValidationComparison{}, domain.ErrNotFound
	}
	if base.ProjectID != id || target.ProjectID != id {
		return domain.ValidationComparison{}, domain.ErrInvalid
	}
	return domain.CompareValidationRuns(*base, *target), nil
}
