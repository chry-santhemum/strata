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

func TestListChildProjectsByRecentActivityUsesNewestSubtreeUpdate(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	if err := store.SaveProjects([]Project{
		{Path: "alpha", CreatedAt: base.Add(1 * time.Hour)},
		{Path: "archive", CreatedAt: base.Add(2 * time.Hour)},
		{Path: "personal", CreatedAt: base.Add(3 * time.Hour)},
		{Path: "work", CreatedAt: base.Add(4 * time.Hour)},
		{Path: "work/api", CreatedAt: base.Add(5 * time.Hour)},
	}); err != nil {
		t.Fatalf("save projects: %v", err)
	}

	children, err := store.listChildProjectsByRecentActivity("", []Record{
		{
			ID:          "personal-record",
			ProjectPath: "personal",
			StartedAt:   base.Add(6 * time.Hour),
			EndedAt:     base.Add(7 * time.Hour),
		},
		{
			ID:          "work-record",
			ProjectPath: "work/api",
			StartedAt:   base.Add(8 * time.Hour),
			EndedAt:     base.Add(9 * time.Hour),
		},
	}, &TimerState{
		Status:        TimerStatusRunning,
		ProjectPath:   "archive",
		StartedAt:     base.Add(10 * time.Hour),
		LastStartedAt: base.Add(10 * time.Hour),
	})
	if err != nil {
		t.Fatalf("list recent child projects: %v", err)
	}

	assertProjectOrder(t, children, "archive", "work", "personal", "alpha")
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

func assertProjectOrder(t *testing.T, projects []Project, paths ...string) {
	t.Helper()
	if len(projects) != len(paths) {
		t.Fatalf("project count = %d, want %d: %+v", len(projects), len(paths), projects)
	}
	for i, path := range paths {
		if projects[i].Path != path {
			t.Fatalf("project %d = %q, want %q in %+v", i, projects[i].Path, path, projects)
		}
	}
}
