package approvalgatestalecache

import (
	"testing"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/workflow"
)

func TestApprovalGateRefreshesAfterProjectMutation(t *testing.T) {
	store, err := repository.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store, validation.NewEngine())
	created, err := service.CreateProject(workflow.CreateInput{
		ProjectID: "stale-gate", Title: "门禁缓存复现", VenueZone: "一层展厅",
		Width: 300, Height: 200, Gap: 3, Standard: "GB/T 15720",
		Reviewer: "reviewer", RequestID: "create-gate",
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze(created.Project.ProjectID, "maker", "freeze-gate", created.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	proof := domain.ProofRevision{
		ProofID: "proof-gate", SourceDigest: "sha256:approval-gate",
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
	submitted, err := service.SubmitProof(created.Project.ProjectID, "maker", "submit-gate", frozen.Project.Revision, proof)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := service.Validate(created.Project.ProjectID, "reviewer", "validate-gate", submitted.Project.Revision, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ApprovalGate(created.Project.ProjectID, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Ready || first.Revision != checked.Project.Revision {
		t.Fatalf("初始门禁应可批准: %#v", first)
	}

	mutated, err := service.AddFinding(created.Project.ProjectID, "reviewer", "add-blocker", "严重", "新增人工阻断问题", []string{"L1"}, checked.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if mutated.Project.Status != domain.StatusRemediation {
		t.Fatalf("新增问题后项目状态应为待整改，得到 %s", mutated.Project.Status)
	}

	refreshed, err := service.ApprovalGate(created.Project.ProjectID, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Revision != mutated.Project.Revision || refreshed.Ready {
		t.Fatalf("项目变化后仍返回旧批准门禁: gate_revision=%d project_revision=%d ready=%v", refreshed.Revision, mutated.Project.Revision, refreshed.Ready)
	}
}
