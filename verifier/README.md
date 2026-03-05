# verifier

Core abstractions for the ZTS validator chain.

## Key Types

| File | What |
|------|------|
| `verifier.go` | `Interface` — single method `Handle(ctx, request) error` |
| `chain.go` | `Chain(...)` — runs validators in order, short-circuits on first error |
| `handler.go` | `ToHandler(v)` — adapts a `verifier.Interface` into `types.VerifyHandler` |
| `tracker.go` | `Tracker` — per-request state: validators run, failures |
| `instrumented.go` | `Instrumented(name, v, m)` — wraps a validator with metrics (counter + histogram) |
| `resolver_middleware.go` | `ResolverMiddleware` — resolves pipeline YAML, stores result in context via `PipelineHolder` |

## How the Chain Works

```
handleVerify()
  └─ Tracker created & injected into context
       └─ Chain([resolver_middleware?, require_account, step_lookup, dispatcher, webhook])
            ├─ each validator calls Handle(ctx, request)
            ├─ Instrumented wrapper records metrics + updates Tracker
            └─ first error → chain stops, task denied
```

## Writing a New Middleware

Implement `Interface`:

```go
type MyMiddleware struct{}

func (m *MyMiddleware) Handle(ctx context.Context, req types.VerifyRequest) error {
    // return nil to pass, return error to block
    return nil
}
```

Or use the functional shortcut:

```go
verifier.From(func(ctx context.Context, req types.VerifyRequest) error {
    return nil
})
```

## Tracker

The `Tracker` is injected into the context at request start. Any validator can read/write it:

```go
t := verifier.TrackerFrom(ctx)
t.Record("my_validator", false)              // log a pass
```

## PipelineHolder

The resolver middleware stores the resolved pipeline in a `PipelineHolder` in the context, separate from the `Tracker`. Downstream validators access it via:

```go
rp := verifier.ResolvedPipelineFrom(ctx)     // *resolver.ResolvedPipeline or nil
```
