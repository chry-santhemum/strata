package strata

import (
	"errors"
	"fmt"
	"io"
	"time"
)

func Main(args []string, stdout, stderr io.Writer) int {
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
		runErr = runStop(store, args[1:], stdout)
	case "ls":
		runErr = runList(store, args[1:], stdout)
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
	fmt.Fprintln(stdout, "Run strata start to resume, or strata stop to save it.")
	return nil
}

func runStop(store *Store, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: strata stop")
	}
	result, err := store.StopTimer(time.Now())
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

	children, err := store.ListChildProjects(projectPath)
	if err != nil {
		return err
	}
	records, err := store.LoadRecords()
	if err != nil {
		return err
	}

	title := "/"
	if projectPath != "" {
		title = "/" + projectPath
	}
	fmt.Fprintf(stdout, "%s\n\n", title)

	fmt.Fprintln(stdout, "Folders:")
	if len(children) == 0 {
		fmt.Fprintln(stdout, "  none")
	} else {
		for _, child := range children {
			fmt.Fprintf(stdout, "  %s/\n", BaseProjectName(child.Path))
		}
	}

	fmt.Fprintln(stdout, "\nFiles:")
	wroteRecord := false
	for _, record := range records {
		if record.ProjectPath != projectPath {
			continue
		}
		wroteRecord = true
		fmt.Fprintf(stdout, "  %s  %s  %s\n", FormatLocalTime(record.StartedAt), FormatDuration(time.Duration(record.DurationNanos)), record.ID)
	}
	if !wroteRecord {
		fmt.Fprintln(stdout, "  none")
	}
	return nil
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
	fmt.Fprintln(w, "  strata stop")
	fmt.Fprintln(w, "  strata ls [project]")
	fmt.Fprintln(w, "  strata mv <source-project> <target-project>")
}
