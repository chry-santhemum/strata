package strata

import (
	"testing"
	"time"
)

func TestRenameProjectRenamesProjectsRecordsAndState(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	mustCreateProject(t, store, "work", "api")
	mustCreateProject(t, store, "", "personal")

	if err := store.SaveRecords([]Record{
		{ID: "one", ProjectPath: "work/api", DurationNanos: int64(5 * time.Minute)},
		{ID: "two", ProjectPath: "personal", DurationNanos: int64(2 * time.Minute)},
	}); err != nil {
		t.Fatalf("save records: %v", err)
	}
	if err := store.SaveState(&TimerState{
		Status:        TimerStatusRunning,
		ProjectPath:   "work/api",
		StartedAt:     time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
		LastStartedAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := store.RenameProject("work", "personal/work"); err != nil {
		t.Fatalf("rename project: %v", err)
	}

	projects, err := store.LoadProjects()
	if err != nil {
		t.Fatalf("load projects: %v", err)
	}
	assertProjectExists(t, projects, "personal/work")
	assertProjectExists(t, projects, "personal/work/api")
	assertProjectMissing(t, projects, "work")

	records, err := store.LoadRecords()
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if records[0].ProjectPath != "personal/work/api" {
		t.Fatalf("record project path = %q, want personal/work/api", records[0].ProjectPath)
	}
	if records[1].ProjectPath != "personal" {
		t.Fatalf("unmoved record project path = %q, want personal", records[1].ProjectPath)
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.ProjectPath != "personal/work/api" {
		t.Fatalf("state project path = %q, want personal/work/api", state.ProjectPath)
	}
}

func TestRenameProjectRejectsExistingTarget(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	mustCreateProject(t, store, "", "personal")

	if err := store.RenameProject("work", "personal"); err == nil {
		t.Fatal("rename succeeded, want target-exists error")
	}
}

func mustCreateProject(t *testing.T, store *Store, parent, name string) {
	t.Helper()
	if _, err := store.CreateProject(parent, name); err != nil {
		t.Fatalf("create project %q under %q: %v", name, parent, err)
	}
}

func assertProjectExists(t *testing.T, projects []Project, path string) {
	t.Helper()
	if !projectExists(projects, path) {
		t.Fatalf("project %q missing from %+v", path, projects)
	}
}

func assertProjectMissing(t *testing.T, projects []Project, path string) {
	t.Helper()
	if projectExists(projects, path) {
		t.Fatalf("project %q unexpectedly present in %+v", path, projects)
	}
}
