package stale_rule_evidence_test

import (
	"testing"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/workflow"
)

type memoryRepository struct {
	aggregate domain.Aggregate
}

func (r *memoryRepository) Create(a domain.Aggregate, _ repository.Event) error {
	r.aggregate = a
	return nil
}

func (r *memoryRepository) Load(id string) (domain.Aggregate, error) {
	if r.aggregate.Project.ProjectID != id {
		return domain.Aggregate{}, domain.ErrNotFound
	}
	return r.aggregate, nil
}

func (r *memoryRepository) Save(a domain.Aggregate, expected int64, _ repository.Event) error {
	if r.aggregate.Project.Revision != expected {
		return domain.ErrConflict
	}
	r.aggregate = a
	return nil
}

func (r *memoryRepository) List() ([]domain.Aggregate, error) {
	return []domain.Aggregate{r.aggregate}, nil
}

func proof(id string, sequence int, cells string, bendY float64) domain.ProofRevision {
	return domain.ProofRevision{
		ProofID:      id,
		ProjectID:    "evidence-project",
		Sequence:     sequence,
		SourceDigest: "source-digest-" + id,
		Landmarks: []domain.Landmark{
			{ID: "L1", Name: "入口", X: 30, Y: 30, SymbolID: "S1", LabelID: "B1"},
			{ID: "L2", Name: "展厅", X: 180, Y: 30, SymbolID: "S2", LabelID: "B2"},
		},
		PathSegments: []domain.PathSegment{{
			ID: "P1", FromLandmarkID: "L1", ToLandmarkID: "L2",
			Points: []domain.Point{{X: 30, Y: 30}, {X: 100, Y: bendY}, {X: 180, Y: 30}},
		}},
		TactileSymbols: []domain.TactileSymbol{
			{ID: "S1", LegendKey: "entrance", X: 30, Y: 30, RadiusMM: 4},
			{ID: "S2", LegendKey: "gallery", X: 180, Y: 30, RadiusMM: 4},
		},
		BrailleLabels: []domain.BrailleLabel{
			{ID: "B1", Text: "入口", Cells: cells, X: 30, Y: 55, WidthMM: 12, HeightMM: 8},
			{ID: "B2", Text: "展厅", Cells: "⠉⠙", X: 180, Y: 55, WidthMM: 12, HeightMM: 8},
		},
		LegendEntries: []domain.LegendEntry{{Key: "entrance", Meaning: "入口"}, {Key: "gallery", Meaning: "展厅"}},
		SubmittedBy:   "maker-1",
		SubmittedAt:   time.Unix(int64(sequence), 0).UTC(),
	}
}

func passedResults() []domain.RuleResult {
	results := make([]domain.RuleResult, 0, len(validation.AllRules))
	for _, code := range validation.AllRules {
		results = append(results, domain.RuleResult{RuleCode: code, Passed: true, Severity: "信息"})
	}
	return results
}

func TestLatestFailureCannotBeResurrectedByOlderPass(t *testing.T) {
	first := proof("proof-1", 1, "⠁⠃", 30)
	second := proof("proof-2", 2, "not-braille", 30)
	third := proof("proof-3", 3, "not-braille", 45)
	repo := &memoryRepository{aggregate: domain.Aggregate{
		Project: domain.MapProject{
			ProjectID:       "evidence-project",
			Title:           "规则证据复验",
			VenueZone:       "一层展厅",
			SheetWidthMM:    300,
			SheetHeightMM:   200,
			MinimumGapMM:    3,
			BrailleStandard: "GB/T 15720",
			ReviewerID:      "reviewer-1",
			Status:          domain.StatusChecking,
			Revision:        7,
		},
		Proofs: []domain.ProofRevision{first, second, third},
		Runs: []domain.ValidationRun{
			{RunID: "run-1", ProjectID: "evidence-project", ProofID: first.ProofID, ProofSequence: 1, RuleVersion: validation.RuleVersion, Results: passedResults()},
			{RunID: "run-2", ProjectID: "evidence-project", ProofID: second.ProofID, ProofSequence: 2, RuleVersion: validation.RuleVersion, Results: []domain.RuleResult{
				{RuleCode: validation.RuleBraille, Passed: false, Severity: "严重", ElementRefs: []string{"B1"}},
				{RuleCode: validation.RuleBoundary, Passed: true},
				{RuleCode: validation.RuleDuplicate, Passed: true},
				{RuleCode: validation.RuleConnectivity, Passed: true},
				{RuleCode: validation.RuleSpacing, Passed: true},
			}},
		},
		Findings: []domain.ReviewFinding{{
			FindingID: "finding-braille", ProjectID: "evidence-project", ProofID: second.ProofID,
			Source: "rule", RuleCode: validation.RuleBraille, Status: domain.FindingRejected,
		}},
		Idempotency: map[string]domain.IdempotencyRecord{},
	}}
	service := workflow.New(repo, validation.NewEngine())

	got, err := service.Validate("evidence-project", "reviewer-1", "validate-proof-3", 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Status != domain.StatusRemediation {
		gate, gateErr := service.ApprovalGate("evidence-project", "reviewer-1")
		if gateErr != nil {
			t.Fatal(gateErr)
		}
		t.Fatalf("TestLatestFailureCannotBeResurrectedByOlderPass: 最新 BRAILLE_CELLS 失败仍未被复验，项目却进入 %q，批准门禁 ready=%v", got.Project.Status, gate.Ready)
	}
}
