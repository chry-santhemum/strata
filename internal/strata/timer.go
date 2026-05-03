package strata

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type StartResult struct {
	ProjectPath string
	Resumed     bool
}

type PauseResult struct {
	ProjectPath string
	Elapsed     time.Duration
}

type StopResult struct {
	Record Record
	Totals []ProjectTotal
}

func (s *Store) StartTimer(projectPath string, now time.Time) (StartResult, error) {
	state, err := s.LoadState()
	if err != nil {
		return StartResult{}, err
	}

	if state != nil {
		switch state.Status {
		case TimerStatusRunning:
			return StartResult{}, fmt.Errorf("timer already running on %s", state.ProjectPath)
		case TimerStatusPaused:
			if stringsMatchProject(projectPath, state.ProjectPath) {
				state.Status = TimerStatusRunning
				state.LastStartedAt = now.UTC()
				if err := s.SaveState(state); err != nil {
					return StartResult{}, err
				}
				return StartResult{ProjectPath: state.ProjectPath, Resumed: true}, nil
			}
			return StartResult{}, fmt.Errorf("timer is paused on %s; stop it before starting another project", state.ProjectPath)
		default:
			return StartResult{}, fmt.Errorf("unknown timer state %q", state.Status)
		}
	}

	cleanPath, err := NormalizeProjectPath(projectPath)
	if err != nil {
		return StartResult{}, err
	}
	exists, err := s.ProjectExists(cleanPath)
	if err != nil {
		return StartResult{}, err
	}
	if !exists {
		return StartResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, cleanPath)
	}

	state = &TimerState{
		Status:        TimerStatusRunning,
		ProjectPath:   cleanPath,
		StartedAt:     now.UTC(),
		LastStartedAt: now.UTC(),
	}
	if err := s.SaveState(state); err != nil {
		return StartResult{}, err
	}
	return StartResult{ProjectPath: cleanPath}, nil
}

func (s *Store) PauseTimer(now time.Time) (PauseResult, error) {
	state, err := s.LoadState()
	if err != nil {
		return PauseResult{}, err
	}
	if state == nil {
		return PauseResult{}, errors.New("no active timer")
	}
	if state.Status == TimerStatusPaused {
		return PauseResult{}, fmt.Errorf("timer already paused on %s", state.ProjectPath)
	}
	if state.Status != TimerStatusRunning {
		return PauseResult{}, fmt.Errorf("unknown timer state %q", state.Status)
	}

	elapsed := elapsedForState(*state, now)
	state.Status = TimerStatusPaused
	state.AccumulatedNanos = int64(elapsed)
	state.LastStartedAt = time.Time{}
	if err := s.SaveState(state); err != nil {
		return PauseResult{}, err
	}
	return PauseResult{ProjectPath: state.ProjectPath, Elapsed: elapsed}, nil
}

func (s *Store) StopTimer(now time.Time) (StopResult, error) {
	state, err := s.LoadState()
	if err != nil {
		return StopResult{}, err
	}
	if state == nil {
		return StopResult{}, errors.New("no active timer")
	}
	if state.Status != TimerStatusRunning && state.Status != TimerStatusPaused {
		return StopResult{}, fmt.Errorf("unknown timer state %q", state.Status)
	}

	duration := elapsedForState(*state, now)
	record := Record{
		ID:            newRecordID(now),
		ProjectPath:   state.ProjectPath,
		StartedAt:     state.StartedAt,
		EndedAt:       now.UTC(),
		DurationNanos: int64(duration),
	}

	if err := s.AppendRecord(record); err != nil {
		return StopResult{}, err
	}
	if err := s.ClearState(); err != nil {
		return StopResult{}, err
	}

	records, err := s.LoadRecords()
	if err != nil {
		return StopResult{}, err
	}
	return StopResult{
		Record: record,
		Totals: totalsForProject(records, state.ProjectPath),
	}, nil
}

func elapsedForState(state TimerState, now time.Time) time.Duration {
	elapsed := time.Duration(state.AccumulatedNanos)
	if state.Status == TimerStatusRunning && !state.LastStartedAt.IsZero() {
		elapsed += now.UTC().Sub(state.LastStartedAt)
	}
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func totalsForProject(records []Record, projectPath string) []ProjectTotal {
	paths := AncestorProjectPaths(projectPath)
	totals := make([]ProjectTotal, 0, len(paths))
	for _, path := range paths {
		var total time.Duration
		for _, record := range records {
			if isProjectInSubtree(record.ProjectPath, path) {
				total += time.Duration(record.DurationNanos)
			}
		}
		totals = append(totals, ProjectTotal{
			ProjectPath: path,
			Duration:    total,
		})
	}
	return totals
}

func stringsMatchProject(input, projectPath string) bool {
	if input == "" {
		return true
	}
	clean, err := NormalizeProjectPath(input)
	if err != nil {
		return false
	}
	return clean == projectPath
}

func newRecordID(now time.Time) string {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return now.UTC().Format("20060102T150405.000000000Z")
	}
	return now.UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random)
}
