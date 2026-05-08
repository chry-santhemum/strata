package strata

import "time"

const (
	TimerStatusRunning = "running"
	TimerStatusPaused  = "paused"
)

type Project struct {
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type Record struct {
	ID            string     `json:"id"`
	ProjectPath   string     `json:"project_path"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       time.Time  `json:"ended_at"`
	DurationNanos int64      `json:"duration_nanos"`
	Plan          *FocusPlan `json:"plan,omitempty"`
}

type TimerState struct {
	Status           string     `json:"status"`
	ProjectPath      string     `json:"project_path"`
	StartedAt        time.Time  `json:"started_at"`
	LastStartedAt    time.Time  `json:"last_started_at,omitempty"`
	AccumulatedNanos int64      `json:"accumulated_nanos"`
	Plan             *FocusPlan `json:"plan,omitempty"`
}

type FocusPlan struct {
	PlannedDuration      string `json:"planned_duration,omitempty"`
	ImmediateNextActions string `json:"immediate_next_actions,omitempty"`
	ExpectedOutputs      string `json:"expected_outputs,omitempty"`
}

type ProjectTotal struct {
	ProjectPath string
	Duration    time.Duration
}

type projectsFile struct {
	Projects []Project `json:"projects"`
}
