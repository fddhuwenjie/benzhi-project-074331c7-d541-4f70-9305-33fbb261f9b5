package large_event_reload_test

import (
	"strings"
	"testing"

	"tactile-atlas-gate/internal/repository"
	"tactile-atlas-gate/internal/workflow"
)

func TestLargeEventRemainsReadableAfterCommit(t *testing.T) {
	dataDir := t.TempDir()
	store, err := repository.NewFileStore(dataDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	service := workflow.New(store, nil)
	created, err := service.CreateProject(workflow.CreateInput{
		ProjectID: "large-event-project",
		Title:     strings.Repeat("x", 70<<10),
		VenueZone: "一层展厅",
		Width:     300,
		Height:    200,
		Gap:       3,
		Standard:  "GB/T 15720",
		Reviewer:  "reviewer-1",
		RequestID: "request-large-event",
	})
	if err != nil {
		t.Fatalf("commit large event: %v", err)
	}

	restarted, err := repository.NewFileStore(dataDir)
	if err != nil {
		t.Fatalf("restart store: %v", err)
	}
	restartedService := workflow.New(restarted, nil)
	loaded, err := restartedService.Get(created.Project.ProjectID)
	if err != nil {
		t.Fatalf("committed project cannot be reloaded: %v", err)
	}
	if loaded.Project.ProjectID != created.Project.ProjectID {
		t.Fatalf("reloaded wrong project: got %q", loaded.Project.ProjectID)
	}
}
