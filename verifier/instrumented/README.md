# verifier/instrumented

An **instrumentation wrapper** that adds metrics recording and per-request tracking to any validator. When the validator chain is built from config, each validator is wrapped with `Wrap` so that every evaluation is automatically timed, counted, and recorded in the request's tracker — without the validator itself needing to know about metrics or tracking.

## Folder Structure

```
instrumented/
├── instrumented.go   Wrap() — decorates a validator with metrics and tracker updates
└── tracker.go        Tracker — per-request state collector, stored in context
```

## How It Works

### Wrap

`Wrap(name, validator, metrics)` returns a new `verifier.Interface` that:

1. Calls the inner validator's `Handle`.
2. Records the duration and pass/fail result via `metrics.ValidatorEvaluationsTotal` and `metrics.ValidatorDuration`.
3. If the validator denied the task, also increments `metrics.BlockedTasksTotal` with the account, task type, and validator name.
4. Updates the `Tracker` in the context (if present) with the validator name and whether it failed.

### Tracker

`Tracker` is a lightweight struct stored in the request context. It collects:

- **`validatorsRun`** — ordered list of validator names that executed.
- **`failedValidator`** — the name of the first validator that denied the task (empty if all passed).

These results are read after the chain completes and included in the audit record, giving a clear picture of what happened during each authorization decision.

| Function | Purpose |
|----------|---------|
| `NewTracker()` | Create a new tracker |
| `WithTracker(ctx, t)` | Store the tracker in context |
| `TrackerFrom(ctx)` | Retrieve the tracker from context |
| `t.Record(name, failed)` | Record a validator evaluation |
| `t.Results()` | Get the final validators-run list and failed validator name |
