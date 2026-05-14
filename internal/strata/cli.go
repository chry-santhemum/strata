package strata

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	store, err := NewStoreFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "strata: %v\n", err)
		return 1
	}

	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	var runErr error
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "project":
		runErr = runProject(store, args[1:])
	case "start":
		runErr = runStart(store, args[1:], stdout)
	case "pause":
		runErr = runPause(store, args[1:], stdout)
	case "stop":
		runErr = runStop(store, args[1:], stdin, stdout)
	case "discard":
		runErr = runDiscard(store, args[1:], stdin, stdout)
	case "ls":
		runErr = runList(store, args[1:], stdout)
	case "log":
		runErr = runLog(store, args[1:], stdout)
	case "show":
		runErr = runShow(store, args[1:], stdout)
	case "mv":
		runErr = runMove(store, args[1:], stdout)
	default:
		runErr = fmt.Errorf("unknown command %q", args[0])
	}

	if runErr != nil {
		fmt.Fprintf(stderr, "strata: %v\n", runErr)
		return 1
	}
	return 0
}

func runProject(store *Store, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: strata project")
	}
	return RunProjectManager(store)
}

func runStart(store *Store, args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: strata start [project]")
	}

	projectPath := ""
	if len(args) == 1 {
		projectPath = args[0]
	}

	state, err := store.LoadState()
	if err != nil {
		return err
	}
	if state != nil {
		result, err := store.StartTimer(projectPath, time.Now())
		if err != nil {
			return err
		}
		if result.Resumed {
			fmt.Fprintf(stdout, "Resumed timer on %s.\n", result.ProjectPath)
			return nil
		}
	}

	if projectPath == "" {
		selectedPath, err := RunStartPicker(store)
		if errors.Is(err, ErrPickerCancelled) {
			fmt.Fprintln(stdout, "No timer started.")
			return nil
		}
		if err != nil {
			return err
		}
		projectPath = selectedPath
	}

	result, err := store.StartTimer(projectPath, time.Now())
	if err != nil {
		return err
	}
	if err := RunFocusPlanPrompt(store); err != nil {
		if errors.Is(err, ErrFocusPlanAborted) {
			fmt.Fprintln(stdout, "Start aborted.")
			return nil
		}
		return err
	}
	fmt.Fprintf(stdout, "Started timer on %s.\n", result.ProjectPath)
	return nil
}

func runPause(store *Store, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: strata pause")
	}
	result, err := store.PauseTimer(time.Now())
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Paused timer on %s at %s.\n", result.ProjectPath, FormatDuration(result.Elapsed))
	fmt.Fprintln(stdout)
	writePlanBlock(stdout, result.Plan)
	fmt.Fprintln(stdout, "Run strata start to resume, or strata stop to save it.")
	return nil
}

func runStop(store *Store, args []string, stdin io.Reader, stdout io.Writer) error {
	adjustment, hasAdjustment, err := parseStopAdjustment(args)
	if err != nil {
		return err
	}

	plan, err := store.PlanStop(time.Now(), adjustment)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Stop current %s timer on %s?\n", plan.Status, plan.ProjectPath)
	fmt.Fprintf(stdout, "Duration: %s\n", FormatDuration(plan.Duration))
	if hasAdjustment {
		fmt.Fprintf(stdout, "Adjustment: %s\n", FormatSignedDuration(plan.Adjustment))
	}
	fmt.Fprintln(stdout)
	writePlanBlock(stdout, plan.Plan)
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, "Save this record? [y/N]: ")

	confirmed, err := readConfirmation(stdin)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(stdout, "Stop cancelled.")
		return nil
	}

	result, err := store.CommitStop(plan, time.Now())
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Recorded: %s\n", FormatDuration(time.Duration(result.Record.DurationNanos)))
	for index, total := range result.Totals {
		label := "Project total"
		if index > 0 {
			label = "Parent total"
		}
		fmt.Fprintf(stdout, "%s: %s  %s\n", label, total.ProjectPath, FormatDuration(total.Duration))
	}
	return nil
}

func parseStopAdjustment(args []string) (time.Duration, bool, error) {
	if len(args) == 0 {
		return 0, false, nil
	}
	if len(args) > 1 {
		return 0, false, errors.New("usage: strata stop [negative-hour-adjustment]")
	}

	hours, err := strconv.ParseFloat(args[0], 64)
	if err != nil || math.IsNaN(hours) || math.IsInf(hours, 0) {
		return 0, false, errors.New("stop adjustment must be a negative number of hours")
	}
	if hours >= 0 {
		return 0, false, errors.New("stop adjustment must be negative")
	}
	return time.Duration(hours * float64(time.Hour)), true, nil
}

func runDiscard(store *Store, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: strata discard")
	}

	state, err := store.LoadState()
	if err != nil {
		return err
	}
	if state == nil {
		return errors.New("no active timer")
	}
	if state.Status != TimerStatusRunning && state.Status != TimerStatusPaused {
		return fmt.Errorf("unknown timer state %q", state.Status)
	}

	elapsed := elapsedForState(*state, time.Now())
	fmt.Fprintf(stdout, "Discard current %s timer on %s (%s)? [y/N]: ", state.Status, state.ProjectPath, FormatDuration(elapsed))

	confirmed, err := readConfirmation(stdin)
	if err != nil {
		return err
	}
	if confirmed {
		if err := store.ClearState(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Discarded current session on %s.\n", state.ProjectPath)
	} else {
		fmt.Fprintln(stdout, "Discard cancelled.")
	}
	return nil
}

func readConfirmation(stdin io.Reader) (bool, error) {
	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func runList(store *Store, args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: strata ls [project]")
	}

	projectPath := ""
	if len(args) == 1 {
		var err error
		projectPath, err = NormalizeOptionalProjectPath(args[0])
		if err != nil {
			return err
		}
	}

	records, err := store.LoadRecords()
	if err != nil {
		return err
	}
	state, err := store.LoadState()
	if err != nil {
		return err
	}
	children, err := store.listChildProjectsByRecentActivity(projectPath, records, state)
	if err != nil {
		return err
	}

	title := "/"
	if projectPath != "" {
		title = "/" + projectPath
	}
	fmt.Fprintf(stdout, "%s\n\n", title)

	rows := listRows(children, records, projectPath)
	nameWidth := 0
	for _, row := range rows {
		if len(row.name) > nameWidth {
			nameWidth = len(row.name)
		}
	}
	for _, row := range rows {
		fmt.Fprintf(stdout, "%-*s  %s\n", nameWidth, row.name, FormatDuration(row.duration))
	}
	return nil
}

func runLog(store *Store, args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: strata log [project]")
	}

	projectPath := ""
	if len(args) == 1 {
		var err error
		projectPath, err = NormalizeOptionalProjectPath(args[0])
		if err != nil {
			return err
		}
	}

	records, err := store.LoadRecords()
	if err != nil {
		return err
	}
	records = filterRecordsForProject(records, projectPath)
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	if len(records) == 0 {
		fmt.Fprintln(stdout, "No records.")
		return nil
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "%s  %-8s  %-16s  %s\n", FormatLocalTime(record.StartedAt), FormatDuration(time.Duration(record.DurationNanos)), displayProjectPath(record.ProjectPath), record.ID)
	}
	return nil
}

func runShow(store *Store, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: strata show <record-id>")
	}

	records, err := store.LoadRecords()
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.ID != args[0] {
			continue
		}
		fmt.Fprintf(stdout, "Project: %s\n", displayProjectPath(record.ProjectPath))
		fmt.Fprintf(stdout, "Started: %s\n", FormatLocalTime(record.StartedAt))
		fmt.Fprintf(stdout, "Duration: %s\n\n", FormatDuration(time.Duration(record.DurationNanos)))
		writePlanBlock(stdout, record.Plan)
		return nil
	}
	return fmt.Errorf("record not found: %s", args[0])
}

func filterRecordsForProject(records []Record, projectPath string) []Record {
	if projectPath == "" {
		return records
	}
	filtered := make([]Record, 0, len(records))
	for _, record := range records {
		if isProjectInSubtree(record.ProjectPath, projectPath) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func displayProjectPath(projectPath string) string {
	if projectPath == "" {
		return "/"
	}
	return projectPath
}

type listRow struct {
	name     string
	duration time.Duration
}

func listRows(children []Project, records []Record, projectPath string) []listRow {
	rows := make([]listRow, 0, len(children)+1)
	for _, child := range children {
		rows = append(rows, listRow{
			name:     BaseProjectName(child.Path) + "/",
			duration: totalDurationInSubtree(records, child.Path),
		})
	}
	rows = append(rows, listRow{
		name:     "(unk.)",
		duration: totalDurationDirectlyInProject(records, projectPath),
	})
	return rows
}

func totalDurationInSubtree(records []Record, projectPath string) time.Duration {
	var total time.Duration
	for _, record := range records {
		if isProjectInSubtree(record.ProjectPath, projectPath) {
			total += time.Duration(record.DurationNanos)
		}
	}
	return total
}

func totalDurationDirectlyInProject(records []Record, projectPath string) time.Duration {
	var total time.Duration
	for _, record := range records {
		if record.ProjectPath == projectPath {
			total += time.Duration(record.DurationNanos)
		}
	}
	return total
}

func runMove(store *Store, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: strata mv <source-project> <target-project>")
	}
	if err := store.RenameProject(args[0], args[1]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Renamed %s to %s.\n", args[0], args[1])
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "strata - a small command line time tracker")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  strata project")
	fmt.Fprintln(w, "  strata start [project]")
	fmt.Fprintln(w, "  strata pause")
	fmt.Fprintln(w, "  strata stop [negative-hour-adjustment]")
	fmt.Fprintln(w, "  strata discard")
	fmt.Fprintln(w, "  strata ls [project]")
	fmt.Fprintln(w, "  strata log [project]")
	fmt.Fprintln(w, "  strata show <record-id>")
	fmt.Fprintln(w, "  strata mv <source-project> <target-project>")
}
