# middleware

First-party middleware constructors that wrap the ZTS verify and output handlers. Each constructor returns a `zts.VerifyMiddleware` or `zts.OutputMiddleware` so customers can compose them with their own middlewares via `zts.WithVerifyMiddleware` / `zts.WithOutputMiddleware`.

## Folder Structure

```
middleware/
├── verify/      Middlewares for the /api/verify handler
│   ├── logging.go         Logging — per-request "processing" / outcome log lines
│   ├── metrics.go         Metrics — zts_verify_requests_total + duration histogram
│   ├── metadata.go        MissingMetadata — counts requests with missing zts_metadata fields
│   └── audit.go           Audit — writes audit.Record via the supplied audit.Writer
│
└── output/      Middlewares for the /api/output handler
    ├── logging.go         Logging — per-request "processing" / outcome log lines
    ├── metrics.go         Metrics — zts_output_requests_total
    └── audit.go           Audit — writes audit.OutputRecord via the supplied audit.Writer
```

## How It Works

A middleware is a function that wraps a handler:

```go
type VerifyMiddleware func(next types.VerifyHandler) types.VerifyHandler
type OutputMiddleware func(next types.OutputHandler) types.OutputHandler
```

Middlewares compose like chi: the first one passed to `WithVerifyMiddleware` is **outermost** — it sees the request first and the response last.

```go
srv := zts.NewServer(
    zts.WithVerifyMiddleware(
        verifymw.Logging(),         // outermost — logs full request/response lifecycle
        verifymw.Metrics(emitter),
        verifymw.MissingMetadata(emitter),
        verifymw.Audit(auditWriter),
    ),
    zts.WithOutputMiddleware(
        outputmw.Logging(),
        outputmw.Metrics(emitter),
        outputmw.Audit(auditWriter),
    ),
)
```

### Verify Middlewares

| Constructor | What it does |
|-------------|--------------|
| `verify.Logging()` | Logs `[verify] processing …` on entry and `[verify] authorized / denied / internal error …` on exit, with task_id, account_id, task_type, and duration. |
| `verify.Metrics(m)` | Emits `zts_verify_requests_total` (counter) and `zts_verify_request_duration_seconds` (histogram), each dimensioned by `status` (`authorized` / `unauthorized` / `error`) and `account_id`. |
| `verify.MissingMetadata(m)` | Increments `zts_missing_metadata_total{field=…}` when the request is missing `zts_metadata`, `account_id`, or `task_type`. Does not change the verify outcome. |
| `verify.Audit(w)` | Calls `w.WriteEvent(audit.EventVerify, record, rawPayload)` with the structured `audit.Record` and the raw request body (read from context, set by the HTTP handler). |

### Output Middlewares

| Constructor | What it does |
|-------------|--------------|
| `output.Logging()` | Logs `[output] processing …` on entry and `[output] success / internal error …` on exit, with task_id, account_id, response code, and duration. |
| `output.Metrics(m)` | Emits `zts_output_requests_total` dimensioned by `status` (`success` / `error`) and `account_id`. |
| `output.Audit(w)` | Calls `w.WriteEvent(audit.EventOutput, record, rawPayload)` with the structured `audit.OutputRecord` and the raw request body (read from context). |

### Auto-Applied: Response Timing

The SDK automatically wraps the verify chain with an internal `responseTiming` middleware as the **outermost** layer — it records `startTs` and `endTs` (UTC unix-millis) into the response `Metadata` so the client sees end-to-end server latency. Customers don't list this middleware; it is always present.

## Writing Your Own Middleware

A middleware is just a function:

```go
func Tracing() zts.VerifyMiddleware {
    return func(next types.VerifyHandler) types.VerifyHandler {
        return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
            ctx, span := tracer.Start(ctx, "zts.verify")
            defer span.End()
            return next(ctx, req)
        }
    }
}
```

Pass it to `zts.WithVerifyMiddleware(Tracing(), …)` alongside the first-party middlewares.

## Notes

- All metric/log dimensions use `req.ResolveAccountID()`, `req.ResolveTaskType()`, and the typed accessors (`req.TaskID()`, `req.DelegateID()`, etc.) on `types.VerifyRequest` / `types.OutputRequest` — middlewares never reach into the request DTOs directly.
- `Metrics` and `MissingMetadata` panic if the emitter is nil; `Audit` panics if the writer is nil. This is intentional — these are configuration errors, not runtime conditions.
- The audit middleware calls `WriteEvent` synchronously. The `audit.Writer` implementation decides whether to optimize (spawn a goroutine, batch events, etc.).
