package event_snapshot_atomicity_test

import (
	"errors"
	"testing"
	"time"

	"tactile-atlas-gate/internal/domain"
	"tactile-atlas-gate/internal/repository"
)

type rejectedEventPayload struct{}

func (rejectedEventPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("forced event serialization failure")
}

func TestEventFailureDoesNotCommitSnapshot(t *testing.T) {
	store, err := repository.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	created := domain.Aggregate{
		Project: domain.MapProject{
			ProjectID:       "atomic-project",
			Title:           "提交前标题",
			VenueZone:       "一层展厅",
			SheetWidthMM:    300,
			SheetHeightMM:   200,
			MinimumGapMM:    3,
			BrailleStandard: "GB/T 15720",
			ReviewerID:      "reviewer-1",
			Status:          domain.StatusDraft,
			CreatedAt:       time.Unix(1, 0).UTC(),
		},
		Idempotency: map[string]domain.IdempotencyRecord{},
	}
	if err = store.Create(created, repository.Event{Type: "project.created", Payload: map[string]string{"title": created.Project.Title}}); err != nil {
		t.Fatal(err)
	}

	next := created
	next.Project.Title = "不应提交的标题"
	next.Project.Revision = 1
	err = store.Save(next, 0, repository.Event{Type: "project.updated", ProjectRevision: 1, Payload: rejectedEventPayload{}})
	if err == nil {
		t.Fatal("事件序列化失败未向调用方返回错误")
	}

	loaded, loadErr := store.Load(created.Project.ProjectID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if loaded.Project.Revision != 0 || loaded.Project.Title != created.Project.Title {
		t.Fatalf("TestEventFailureDoesNotCommitSnapshot: Save 返回失败后快照仍从 revision 0 推进为 %d，标题为 %q", loaded.Project.Revision, loaded.Project.Title)
	}
}
