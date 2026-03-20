# Zero-Trust Service (ZTS)

A Go library for building a task authorization layer between the Harness Platform and the Delegate. ZTS provides a pluggable validator chain, optional pipeline template resolution, audit logging, and metrics — all customer-controlled.

## How It Works

```
Harness Manager ──► Delegate ──► ZTS /api/verify ──► allowed / denied
                                      │
                                      ├─ Global validators
                                      ├─ Task-type validators
                                      ├─ Custom validators (webhook)
                                      └─ Pipeline resolver (optional)
```

The Delegate sends each task to ZTS before execution. ZTS runs a chain of validators and returns allow/deny. The chain, resolver, audit, and metrics are all pluggable — bring your own implementations or use the provided defaults.

## Quick Start

```bash
# Prerequisites: Go 1.24+
git clone <repo-url> && cd zero-trust-service

make run-example-zts   # starts on http://localhost:4210

curl -s -X POST http://localhost:4210/api/verify \
  -H "Content-Type: application/json" \
  -d '{"taskPackage":{"delegateTaskId":"t1","accountId":"myAcct","data":{"taskType":"SHELL_SCRIPT_TASK_NG"}}}'
```

For the full production-ready example (Docker, K8s, config, env vars), see [`examples/zts/`](./examples/zts/).

## Core Components

### Validators

Validators run in order: **global → task-type → custom**. First failure blocks the task.

| Type | Scope | Description |
|------|-------|-------------|
| `require_account` | Global | Reject tasks from unlisted accounts |
| `step_lookup` | Global | Log step FQN presence in resolved pipeline |
| `shellscript` | Task-type | Allow only approved shellscript types and commands |
| `image_allowlist` | Task-type | Allow only approved container image prefixes |
| `webhook` | Custom | Forward task to an external policy endpoint |

Custom webhook validators let you add arbitrary policy logic without modifying ZTS — see [`validators/custom/README.md`](./validators/custom/README.md).

### Pipeline Resolver

When enabled, ZTS recursively resolves template references (Pipeline → Stage → Step) using go-scm. Supports GitHub, GitLab, Harness Code, and more. Resolved YAML is available for audit and for downstream validators like `step_lookup`.

See [`resolver/README.md`](./resolver/README.md) for details.

### Audit

Pluggable audit logging. The library defines an `audit.Writer` interface; a file-based implementation is provided in `audit/file/`. See [`audit/README.md`](./audit/README.md).

### Metrics

Pluggable metrics. The library defines `Counter`, `Histogram`, and `Gauge` interfaces; a Prometheus implementation is provided in `metrics/prometheus/`. See [`metrics/README.md`](./metrics/README.md).

## API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/verify` | POST | Authorize a delegate task |
| `/api/output` | POST | Receive task output from delegate |

Admin endpoints (metrics, healthz, audit queries) are hosted separately by the application — see [`examples/zts/`](./examples/zts/) for the reference setup.

## Project Structure

| Directory | README | Description |
|-----------|--------|-------------|
| [`verifier/`](./verifier/) | [README](./verifier/README.md) | Core `Interface`, chain, tracker, instrumentation |
| [`validators/`](./validators/) | [README](./validators/README.md) | Built-in validators and registry |
| [`validators/custom/`](./validators/custom/) | [README](./validators/custom/README.md) | Webhook validator spec and custom validator guide |
| [`metrics/`](./metrics/) | [README](./metrics/README.md) | Metrics interfaces + Prometheus implementation |
| [`resolver/`](./resolver/) | [README](./resolver/README.md) | Pipeline template resolver |
| [`audit/`](./audit/) | [README](./audit/README.md) | Audit logging interface + file-based implementation |
| [`examples/zts/`](./examples/zts/) | [README](./examples/zts/README.md) | Full production ZTS server (config, Docker, K8s) |
| [`examples/basic/`](./examples/basic/) | — | Minimal server with hardcoded validators |
| [`examples/webhook_server/`](./examples/webhook_server/) | — | Sample external policy webhook |
| [`examples/monitoring/`](./examples/monitoring/) | — | Local Prometheus + Grafana dashboard |

## Make Targets

| Target | Description |
|--------|-------------|
| `make run-example-zts` | Run the full ZTS example locally |
| `make run-example-webhook-server` | Run example webhook policy server |
| `make test` | Run all tests |
| `make monitoring-up` | Start local Prometheus + Grafana |
