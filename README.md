# strata

`strata` is a small command line time tracker.

## Commands

```text
strata project
strata start [project]
strata pause
strata stop [negative-hour-adjustment]
strata discard
strata ls [project]
strata log [project]
strata show <record-id>
strata mv <source-project> <target-project>
```

Fresh starts ask a few planning questions after the timer begins. Resuming a paused timer skips those questions.

`strata stop` asks for confirmation before saving a record. You can pass a negative decimal hour adjustment to subtract time from the saved duration:

```text
strata stop -2.5
```

The adjustment is applied after paused time is excluded. If the adjusted duration would be zero or negative, `strata` returns an error and leaves the timer alone.

## Development

```text
go test ./...
go run ./cmd/strata --help
```

written by codex
