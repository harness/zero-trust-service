# metrics

Abstracts metric collection behind interfaces so ZTS is not coupled to a specific metrics backend.

## Package Layout

```
metrics/
├── metrics.go     # Counter, Histogram, Gauge interfaces + Metrics struct + label constants
├── noop.go        # No-op implementation (silent, for testing or when metrics are disabled)
└── prometheus/
    └── prometheus.go  # Prometheus implementation + Handler (RouteRegistrar for /metrics)
```

## Interfaces

| Interface | Method | Description |
|-----------|--------|-------------|
| `Counter` | `Inc(labels ...string)` | Monotonically increasing counter |
| `Histogram` | `Observe(value float64, labels ...string)` | Records observed values (e.g. durations) |
| `Gauge` | `Set(value float64, labels ...string)` | Value that can go up and down |

## Metrics Struct

`Metrics` holds all ZTS metric handles. Every ZTS component receives a `*Metrics` — it never interacts with a specific backend directly.

## Implementations

| Implementation | Constructor | Usage |
|----------------|------------|-------|
| No-op | `metrics.NewNoop()` | Default fallback, testing, metrics disabled |
| Prometheus | `prometheus.New()` | Production use with Prometheus scraping |
| Prometheus (custom registry) | `prometheus.NewWithRegistry(reg)` | Tests needing isolated registries |

The Prometheus package also provides `prometheus.NewHandler()` which implements `RouteRegistrar` and mounts `GET /metrics` on the admin router.

## Adding a New Backend

1. Create a subpackage (e.g. `metrics/otel/`)
2. Implement `Counter`, `Histogram`, `Gauge` interfaces
3. Return a populated `*metrics.Metrics`
4. Optionally implement `RouteRegistrar` if the backend has an HTTP endpoint
