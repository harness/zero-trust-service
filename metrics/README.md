# metrics

The metrics package **decouples ZTS from any specific metrics backend**. It defines a generic `Emitter` interface for counters, histograms, and gauges — ZTS components call these methods without knowing whether the data goes to Prometheus, an OpenTelemetry collector, or nowhere at all.

## Folder Structure

```
metrics/
├── metrics.go       Emitter interface and Dimension type
├── noop.go          No-op implementation (silent, for testing or when metrics are disabled)
└── prometheus/
    └── prometheus.go    Prometheus implementation with lazy metric registration
```

## How to Use

### The Emitter Interface

`Emitter` is a simple three-method interface. Each method accepts a metric name, a value, and zero or more `Dimension` key-value pairs:

```go
m.Counter("zts_verify_requests_total", 1, metrics.Dim("status", "authorized"), metrics.Dim("account_id", "acc-123"))
m.Histogram("zts_verify_request_duration_seconds", 0.003, metrics.Dim("status", "authorized"), metrics.Dim("account_id", "acc-123"))
```

Each package defines its own metric names and dimension key/value strings locally. The metrics package only provides the `Emitter` interface and the `Dimension` type.

### Provided Implementations

| Implementation | Constructor | When to use |
|----------------|------------|-------------|
| No-op | `metrics.NewNoop()` | Testing, or when metrics are disabled |
| Prometheus | `prometheus.New()` | Production with Prometheus scraping |
| Prometheus (custom registry) | `prometheus.NewWithRegistry(reg)` | Tests needing isolated registries |

### Implementing a New Backend

1. Create a subpackage (e.g. `metrics/otel/`).
2. Implement the `Emitter` interface.

```go
func New() metrics.Emitter {
    return &otelEmitter{/* ... */}
}
```
