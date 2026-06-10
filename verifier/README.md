# verifier

The verifier package is the **decision engine** of ZTS. It defines what a verifier is, how verifiers are composed into a chain, and how that chain produces an allow/deny decision for every delegate task. If you want to add new authorization logic to ZTS — whether it's an account allowlist, an image policy, or a call to an external system — this is the abstraction you work with.

The package also handles task-type routing (run different verifiers for different task types), pipeline resolution middleware (fetch and expand template YAML so verifiers can inspect the full pipeline), and per-request tracking (which verifiers ran, which one failed).

## Folder Structure

```
verifier/
├── verifier.go             Interface definition and From() helper
├── chain.go                Chain() — sequential verifier composition
├── handler.go              ToHandler() — adapts Interface into an HTTP handler
├── config.go               VerifiersConfig, VerifierDef (config structs)
├── dispatcher.go           Dispatcher — routes to per-task-type verifier chains
├── resolver_middleware.go  Resolver middleware, PipelineHolder, context helpers
│
├── account/                Built-in verifier: require_account
├── taskdenylist/           Built-in verifier: task_denylist (global denied task types)
├── tasktype/               Built-in verifiers: shellscript, image_allowlist
├── pipeline/               Built-in verifier: step_lookup
└── instrumented/           Wrap() for metrics + Tracker for per-request state
```

## How to Use

### The Core Abstraction

Every verifier implements a single interface:

```go
type Interface interface {
    Handle(ctx context.Context, request types.VerifyRequest) error
}
```

- Return `nil` → the task passes this verifier.
- Return an `error` → the task is denied; the error message becomes the denial reason.

For simple cases, use the functional shortcut instead of defining a struct:

```go
v := verifier.From(func(ctx context.Context, req types.VerifyRequest) error {
    if req.ResolveAccountID() != "allowed-account" {
        return fmt.Errorf("account not permitted")
    }
    return nil
})
```

### Composing Verifiers

`Chain` runs verifiers in order; the first error stops the chain and denies the task:

```go
chain := verifier.Chain(accountValidator, imageValidator, webhookValidator)
```

`ToHandler` adapts a `verifier.Interface` into a `types.VerifyHandler` that the ZTS HTTP server uses:

```go
handler := verifier.ToHandler(chain)
// handler returns VerifyResponse{Allowed: true} on nil,
// or VerifyResponse{Allowed: false, Reason: err.Error()} on error.
```

### Registering a Custom Verifier Type

Define your verifier constructor in its own package:

```go
package mypolicy

type Config struct {
    MaxRisk int
}

func New(cfg Config) (verifier.Interface, error) {
    return verifier.From(func(ctx context.Context, req types.VerifyRequest) error {
        // your logic here
        return nil
    }), nil
}
```

Then supply a `ResolveFunc` to `BuildFromConfig`. The consumer owns the registry and config decoding — for example, a YAML-based application might do:

```go
resolve := func(name string, cfg any) (verifier.Interface, error) {
    switch name {
    case "my_policy":
        // decode cfg (yaml.Node, JSON, etc.) into your typed config
        return mypolicy.New(parsed)
    default:
        return nil, fmt.Errorf("unknown verifier: %s", name)
    }
}
```

See `examples/zts/main.go` for a full registry-based implementation.

### Config-Driven Build

`BuildFromConfig` constructs the full chain from a `ValidatorsConfig` and a `ResolveFunc`:

1. **Global** verifiers — run on every request
2. **Dispatcher** — routes to per-task-type verifier chains
3. **Custom** verifiers — run after task-type verifiers

Each verifier is optionally wrapped with instrumentation (metrics + tracker) via the `WrapFunc` parameter.

### Task-Type Routing

The `Dispatcher` inspects the request's task type and delegates to the matching chain. Unknown task types pass through (no-op).

### Pipeline Resolution

The `Resolver` middleware fetches pipeline YAML from SCM, recursively expands template references, and stores the result in the context via `PipelineHolder`. Downstream verifiers access it with:

```go
rp := verifier.ResolvedPipelineFrom(ctx) // *resolver.ResolvedPipeline or nil
```

### Per-Request Tracking

The `instrumented` subpackage provides:

- **`Wrap(name, v, emitter)`** — decorates a verifier with timing, pass/fail counters, and tracker updates.
- **`Tracker`** — injected into context at request start; records which verifiers ran and which (if any) failed. The results feed into audit records.
