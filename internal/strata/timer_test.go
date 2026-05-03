package strata

import (
	"testing"
	"time"
)

func TestPauseTimeDoesNotCountTowardStoppedRecord(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")

	startedAt := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}
	if _, err := store.PauseTimer(startedAt.Add(10 * time.Minute)); err != nil {
		t.Fatalf("pause timer: %v", err)
	}
	if _, err := store.StartTimer("", startedAt.Add(20*time.Minute)); err != nil {
		t.Fatalf("resume timer: %v", err)
	}
	result, err := store.StopTimer(startedAt.Add(25 * time.Minute))
	if err != nil {
		t.Fatalf("stop timer: %v", err)
	}

	got := time.Duration(result.Record.DurationNanos)
	want := 15 * time.Minute
	if got != want {
		t.Fatalf("recorded duration = %s, want %s", got, want)
	}
}
