# strata

`strata` is a small command line time tracker written in Go. Projects are nested slash paths, and one timer can be active at a time.

## Design choices

- Data is stored in plain files under `~/.strata`.
- Set `STRATA_HOME=/some/path` to use a different data directory.
- Projects live in `projects.json`.
- Finished timer records append to `records.jsonl`.
- The current running or paused timer lives in `state.json`.
- Project paths use `/`, such as `work/client/backend`.
- `strata mv source target` is a rename. It fails if `target` already exists.
- Paused time does not count. `strata start` resumes a paused timer.

## Commands

```text
strata project
strata start [project]
strata pause
strata stop
strata discard
strata ls [project]
strata mv <source-project> <target-project>
```

`strata project` opens the project picker. Select `create new project` to add a project under the current location. Select a project to navigate into it. Press left to go back and `q` to quit.

`strata start` opens the same picker, with a `start timer on current project` option once you have navigated into a project. You can also start directly:

```text
strata start work/client
```

`strata ls` summarizes the current project like a small directory listing. Immediate child projects show their rolled-up total time, including nested records. The `(unk.)` row shows time recorded directly on the current project rather than inside a child project.

`strata discard` works only when a timer is running or paused. It asks for confirmation, then clears the current session without saving a time record.

## Development

```text
go test ./...
go run ./cmd/strata --help
```
