package strata

import (
	"testing"
	"time"
)

func TestStartPickerListsProjectsByRecentActivity(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	if err := store.SaveProjects([]Project{
		{Path: "alpha", CreatedAt: base},
		{Path: "beta", CreatedAt: base},
	}); err != nil {
		t.Fatalf("save projects: %v", err)
	}
	if err := store.SaveRecords([]Record{
		{
			ID:          "old-alpha",
			ProjectPath: "alpha",
			StartedAt:   base.Add(1 * time.Hour),
			EndedAt:     base.Add(2 * time.Hour),
		},
		{
			ID:          "new-beta",
			ProjectPath: "beta",
			StartedAt:   base.Add(3 * time.Hour),
			EndedAt:     base.Add(4 * time.Hour),
		},
	}); err != nil {
		t.Fatalf("save records: %v", err)
	}

	model := newPickerModel(store, pickerPurposeStart)

	var projectItems []pickerItem
	for _, item := range model.items() {
		if item.kind == pickerItemProject {
			projectItems = append(projectItems, item)
		}
	}
	if len(projectItems) != 2 {
		t.Fatalf("project item count = %d, want 2: %+v", len(projectItems), projectItems)
	}
	if projectItems[0].path != "beta" || projectItems[1].path != "alpha" {
		t.Fatalf("project item order = %+v, want beta then alpha", projectItems)
	}
}
