package failed_save_cache_poison

import (
	"os"
	"path/filepath"
	"testing"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/validation"
	"tactile-atlas-gate/internal/workflow"
)

func TestFailedProofSaveDoesNotPoisonIdempotencyCache(t *testing.T) {
	root := t.TempDir()
	store, err := repository.NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store, validation.NewEngine())
	created, err := service.CreateProject(workflow.CreateInput{
		ProjectID: "cache-poison", Title: "缓存隔离复现", VenueZone: "展厅",
		Width: 300, Height: 200, Gap: 3, Standard: "GB/T 15720",
		Reviewer: "reviewer", RequestID: "create-cache-poison",
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze(created.Project.ProjectID, "maker", "freeze-cache-poison", created.Project.Revision)
	if err != nil {
		t.Fatal(err)
	}

	proof := validProof()
	evidencePath := filepath.Join(root, "projects", created.Project.ProjectID, "proofs", "000001-proof-cache-poison.json")
	if err = os.WriteFile(evidencePath, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err = service.SubmitProof(created.Project.ProjectID, "maker", "submit-cache-poison", frozen.Project.Revision, proof); err == nil {
		t.Fatal("预置证据冲突未使首次提交失败")
	}
	replayed, retryErr := service.SubmitProof(created.Project.ProjectID, "maker", "submit-cache-poison", frozen.Project.Revision, proof)
	if retryErr == nil {
		t.Fatalf("失败提交被缓存误判为成功：返回 revision=%d proofs=%d", replayed.Project.Revision, len(replayed.Proofs))
	}

	freshStore, err := repository.NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := freshStore.Load(created.Project.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Project.Revision != frozen.Project.Revision || len(persisted.Proofs) != 0 {
		t.Fatalf("失败提交改变了磁盘快照：revision=%d proofs=%d", persisted.Project.Revision, len(persisted.Proofs))
	}
}

func validProof() domain.ProofRevision {
	return domain.ProofRevision{
		ProofID: "proof-cache-poison", SourceDigest: "sha256:cache-poison-fixture",
		Landmarks: []domain.Landmark{
			{ID: "L1", Name: "入口", X: 30, Y: 30},
			{ID: "L2", Name: "展厅", X: 180, Y: 30},
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
		LegendEntries: []domain.LegendEntry{
			{Key: "entry", Meaning: "入口"},
			{Key: "hall", Meaning: "展厅"},
		},
	}
}
