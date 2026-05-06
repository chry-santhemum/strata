package strata

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRunListSummarizesChildProjectsAndDirectRecords(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	mustCreateProject(t, store, "work", "api")
	mustCreateProject(t, store, "", "personal")

	if err := store.SaveRecords([]Record{
		{ID: "one", ProjectPath: "work", DurationNanos: int64(15 * time.Minute)},
		{ID: "two", ProjectPath: "work/api", DurationNanos: int64(45 * time.Minute)},
		{ID: "three", ProjectPath: "personal", DurationNanos: int64(30 * time.Minute)},
		{ID: "four", ProjectPath: "", DurationNanos: int64(5 * time.Minute)},
	}); err != nil {
		t.Fatalf("save records: %v", err)
	}

	var stdout bytes.Buffer
	if err := runList(store, nil, &stdout); err != nil {
		t.Fatalf("run list: %v", err)
	}

	output := stdout.String()
	assertContains(t, output, "/\n\n")
	assertContains(t, output, "work/      1h 0m")
	assertContains(t, output, "personal/  30m")
	assertContains(t, output, "(unk.)     5m")
	if strings.Contains(output, "Folders:") || strings.Contains(output, "Files:") {
		t.Fatalf("output still contains old section headings:\n%s", output)
	}
}

func TestRunListSummarizesNestedDirectory(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	mustCreateProject(t, store, "work", "api")
	mustCreateProject(t, store, "work/api", "server")

	if err := store.SaveRecords([]Record{
		{ID: "one", ProjectPath: "work", DurationNanos: int64(15 * time.Minute)},
		{ID: "two", ProjectPath: "work/api", DurationNanos: int64(45 * time.Minute)},
		{ID: "three", ProjectPath: "work/api/server", DurationNanos: int64(30 * time.Minute)},
	}); err != nil {
		t.Fatalf("save records: %v", err)
	}

	var stdout bytes.Buffer
	if err := runList(store, []string{"work/api"}, &stdout); err != nil {
		t.Fatalf("run list: %v", err)
	}

	output := stdout.String()
	assertContains(t, output, "/work/api\n\n")
	assertContains(t, output, "server/  30m")
	assertContains(t, output, "(unk.)   45m")
}

func TestRunDiscardCancelledLeavesTimerState(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	startedAt := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}

	var stdout bytes.Buffer
	if err := runDiscard(store, nil, strings.NewReader("n\n"), &stdout); err != nil {
		t.Fatalf("discard: %v", err)
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state == nil {
		t.Fatal("timer state was cleared after cancelled discard")
	}
	assertContains(t, stdout.String(), "Discard cancelled.")
}

func TestRunDiscardConfirmedClearsTimerWithoutRecord(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	startedAt := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}

	var stdout bytes.Buffer
	if err := runDiscard(store, nil, strings.NewReader("yes\n"), &stdout); err != nil {
		t.Fatalf("discard: %v", err)
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state != nil {
		t.Fatalf("timer state still exists after confirmed discard: %+v", state)
	}
	records, err := store.LoadRecords()
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("discard saved records: %+v", records)
	}
	assertContains(t, stdout.String(), "Discarded current session on work.")
}

func TestRunStopCancelledLeavesTimerStateAndRecords(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	startedAt := time.Now().UTC().Add(-30 * time.Minute)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}

	var stdout bytes.Buffer
	if err := runStop(store, nil, strings.NewReader("\n"), &stdout); err != nil {
		t.Fatalf("stop: %v", err)
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state == nil {
		t.Fatal("timer state was cleared after cancelled stop")
	}
	records, err := store.LoadRecords()
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("cancelled stop saved records: %+v", records)
	}

	output := stdout.String()
	assertContains(t, output, "Stop current running timer on work?")
	assertContains(t, output, "Duration:")
	assertContains(t, output, "Stop cancelled.")
	assertNotContains(t, output, "Adjustment:")
}

func TestRunStopConfirmedSavesAdjustedDuration(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	startedAt := time.Now().UTC().Add(-3 * time.Hour)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}

	var stdout bytes.Buffer
	if err := runStop(store, []string{"-1.5"}, strings.NewReader("yes\n"), &stdout); err != nil {
		t.Fatalf("stop: %v", err)
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state != nil {
		t.Fatalf("timer state still exists after confirmed stop: %+v", state)
	}
	records, err := store.LoadRecords()
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1: %+v", len(records), records)
	}
	assertDurationNear(t, time.Duration(records[0].DurationNanos), 90*time.Minute, 2*time.Second)

	output := stdout.String()
	assertContains(t, output, "Stop current running timer on work?")
	assertContains(t, output, "Adjustment: -1h 30m")
	assertContains(t, output, "Recorded: 1h 30m")
}

func TestRunStopRejectsPositiveAdjustment(t *testing.T) {
	store := NewStore(t.TempDir())

	var stdout bytes.Buffer
	err := runStop(store, []string{"1.5"}, strings.NewReader("yes\n"), &stdout)
	if err == nil {
		t.Fatal("stop succeeded, want negative-adjustment error")
	}
	if !strings.Contains(err.Error(), "must be negative") {
		t.Fatalf("error = %q, want negative-adjustment error", err.Error())
	}
}

func TestRunStopRejectsTooLargeAdjustmentBeforePrompt(t *testing.T) {
	store := NewStore(t.TempDir())
	mustCreateProject(t, store, "", "work")
	startedAt := time.Now().UTC().Add(-30 * time.Minute)
	if _, err := store.StartTimer("work", startedAt); err != nil {
		t.Fatalf("start timer: %v", err)
	}

	var stdout bytes.Buffer
	err := runStop(store, []string{"-1"}, strings.NewReader("yes\n"), &stdout)
	if err == nil {
		t.Fatal("stop succeeded, want too-large-adjustment error")
	}
	if !strings.Contains(err.Error(), "zero or negative") {
		t.Fatalf("error = %q, want zero-or-negative error", err.Error())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prompt was written before adjustment error:\n%s", stdout.String())
	}

	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state == nil {
		t.Fatal("timer state was cleared after failed stop")
	}
	records, err := store.LoadRecords()
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("failed stop saved records: %+v", records)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("output missing %q:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("output unexpectedly contains %q:\n%s", needle, haystack)
	}
}

func assertDurationNear(t *testing.T, got, want, tolerance time.Duration) {
	t.Helper()
	if got < want-tolerance || got > want+tolerance {
		t.Fatalf("duration = %s, want %s +/- %s", got, want, tolerance)
	}
}
