package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tactile-atlas-gate/internal/domain"
)

func TestStoreDetectsTruncatedEventFrame(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, err := domain.NewProject("map-1", "测试", "展厅", 300, 200, 3, "GB/T 15720", "reviewer", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.Aggregate{Project: project, Idempotency: map[string]domain.IdempotencyRecord{}}
	if err = store.Create(aggregate, Event{Type: "created"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.root, "projects", "map-1", "events.log")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("{\"sequence\":2"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load("map-1"); err == nil {
		t.Fatal("截断事件帧未被检测")
	}
}

func TestImmutableProofCannotBeChanged(t *testing.T) {
	store, _ := NewFileStore(t.TempDir())
	project, _ := domain.NewProject("map-2", "测试", "展厅", 300, 200, 3, "GB/T 15720", "reviewer", time.Now())
	aggregate := domain.Aggregate{Project: project, Idempotency: map[string]domain.IdempotencyRecord{}}
	if err := store.Create(aggregate, Event{Type: "created"}); err != nil {
		t.Fatal(err)
	}
	proof := domain.ProofRevision{ProofID: "p1", ProjectID: "map-2", Sequence: 1}
	aggregate.Proofs = append(aggregate.Proofs, proof)
	aggregate.Project.Revision = 1
	if err := store.Save(aggregate, 0, Event{Type: "proof"}); err != nil {
		t.Fatal(err)
	}
	aggregate.Proofs[0].SourceDigest = "changed"
	aggregate.Project.Revision = 2
	if err := store.Save(aggregate, 1, Event{Type: "mutate"}); err == nil {
		t.Fatal("已提交校样被允许修改")
	}
}
