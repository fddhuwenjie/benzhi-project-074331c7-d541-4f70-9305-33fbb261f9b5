package late_idempotency_replay_test

import (
	"testing"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/workflow"
)

func TestLateIdempotencyReplayReturnsOriginalResponse(t *testing.T) {
	store, err := repository.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store, validation.NewEngine())
	created, err := service.CreateProject(workflow.CreateInput{
		ProjectID: "late-replay", Title: "迟到重放", VenueZone: "一层",
		Width: 300, Height: 200, Gap: 3, Standard: "GB/T 15720",
		Reviewer: "reviewer", RequestID: "create-late-replay",
	})
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.PreflightSpecification(created.Project.ProjectID, "maker", created.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.FreezeWithPreflight(created.Project.ProjectID, "maker", "freeze-late-replay", created.Project.Revision, preflight.SummaryDigest)
	if err != nil {
		t.Fatal(err)
	}
	proof := domain.ProofRevision{
		ProofID: "proof-late-replay", SourceDigest: "sha256:late-replay",
		Landmarks: []domain.Landmark{
			{ID: "L1", Name: "入口", X: 30, Y: 30, SymbolID: "S1", LabelID: "B1"},
			{ID: "L2", Name: "展厅", X: 180, Y: 30, SymbolID: "S2", LabelID: "B2"},
		},
		PathSegments: []domain.PathSegment{{ID: "P1", FromLandmarkID: "L1", ToLandmarkID: "L2"}},
		TactileSymbols: []domain.TactileSymbol{
			{ID: "S1", LegendKey: "entry", X: 30, Y: 30, RadiusMM: 4},
			{ID: "S2", LegendKey: "hall", X: 180, Y: 30, RadiusMM: 4},
		},
		BrailleLabels: []domain.BrailleLabel{
			{ID: "B1", Text: "入口", Cells: "⠁", X: 30, Y: 55, WidthMM: 12, HeightMM: 8},
			{ID: "B2", Text: "展厅", Cells: "⠃", X: 180, Y: 55, WidthMM: 12, HeightMM: 8},
		},
		LegendEntries: []domain.LegendEntry{{Key: "entry", Meaning: "入口"}, {Key: "hall", Meaning: "展厅"}},
	}
	advanced, err := service.SubmitProof(created.Project.ProjectID, "maker", "proof-after-freeze", frozen.Project.Revision, proof)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Project.Revision == frozen.Project.Revision {
		t.Fatal("测试前置状态没有推进")
	}

	replayed, err := service.FreezeWithPreflight(created.Project.ProjectID, "maker", "freeze-late-replay", created.Project.Revision, preflight.SummaryDigest)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Project.Revision != frozen.Project.Revision || replayed.Project.Status != frozen.Project.Status || len(replayed.Proofs) != 0 {
		t.Fatalf("迟到幂等重放没有返回原始冻结响应: revision=%d status=%s proofs=%d", replayed.Project.Revision, replayed.Project.Status, len(replayed.Proofs))
	}
}
