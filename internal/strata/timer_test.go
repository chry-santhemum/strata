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

func TestPlanStopAppliesAdjustmentAfterPausedTime(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")

	startedAt := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}
	if _, err := store.PauseTimer(startedAt.Add(2 * time.Hour)); err != nil {
		t.Fatalf("pause timer: %v", err)
	}

	plan, err := store.PlanStop(startedAt.Add(5*time.Hour), -90*time.Minute)
	if err != nil {
		t.Fatalf("plan stop: %v", err)
	}

	if plan.Duration != 30*time.Minute {
		t.Fatalf("planned duration = %s, want 30m", plan.Duration)
	}
	if plan.Adjustment != -90*time.Minute {
		t.Fatalf("planned adjustment = %s, want -1h30m", plan.Adjustment)
	}
}

func TestCommitStopCopiesPlanToRecord(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")

	startedAt := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}
	plan := FocusPlan{
		PlannedDuration:      "25m",
		ImmediateNextActions: "Write one test.",
		ExpectedOutputs:      "Passing test.",
	}
	if err := store.SaveCurrentPlan(plan); err != nil {
		t.Fatalf("save focus plan: %v", err)
	}

	stopPlan, err := store.PlanStop(startedAt.Add(30*time.Minute), 0)
	if err != nil {
		t.Fatalf("plan stop: %v", err)
	}
	result, err := store.CommitStop(stopPlan, startedAt.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("commit stop: %v", err)
	}

	if result.Record.Plan == nil {
		t.Fatal("record plan is nil")
	}
	if result.Record.Plan.ExpectedOutputs != "Passing test." {
		t.Fatalf("record expected outputs = %q", result.Record.Plan.ExpectedOutputs)
	}
}
