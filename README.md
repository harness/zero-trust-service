# Zero-Trust Service (ZTS)

ZTS is a **customer-controlled authorization layer** that sits between the Harness Platform and the Delegate. Before a Delegate executes any task (deploy, script, pipeline step, etc.), it sends the task to ZTS for approval. ZTS evaluates the task against a chain of validators and returns an allow or deny decision. This gives platform operators fine-grained, policy-driven control over what the Delegate is permitted to do — independent of the Harness control plane.


```
Harness Manager ──► Delegate ──► ZTS /api/verify ──► allowed / denied
                                      │
                                      ├─ Global validators (apply to all tasks)
                                      ├─ Task-type validators (per task type)
                                      ├─ Custom validators (e.g. webhook)
                                      └─ Pipeline resolver (optional, expands templates)
```

Everything is pluggable — validators, audit backends, metrics collectors, and the pipeline resolver can all be swapped or extended without modifying the core library.

## Folder Structure

```
zero-trust-service/
├── server.go / verify.go / output.go / options.go   Core HTTP server and API handlers
├── types/                                            Request/response DTOs and handler types
│
├── verifier/                 Verifier abstraction — the chain that decides allow/deny
│   ├── account/              Built-in: require_account verifier
│   ├── taskdenylist/         Built-in: task_denylist (global denied task types)
│   ├── tasktype/             Built-in: shellscript and image_allowlist verifiers
│   ├── pipeline/             Built-in: step_lookup verifier
│   └── instrumented/         Metrics + tracking wrapper for verifiers
│
├── middleware/               First-party verify/output middleware constructors
│   ├── verify/               Logging, Metrics, MissingMetadata, Audit (verify chain)
│   └── output/               Logging, Metrics, Audit (output chain)
│
├── audit/                    Audit logging abstraction (Writer interface)
│   └── file/                 File-based audit implementation
│
├── metrics/                  Metrics abstraction (Counter, Histogram, Gauge)
│   └── prometheus/           Prometheus metrics implementation
│
├── resolver/                 Pipeline template resolver (expands templateRef YAML)
│   └── scm/                  SCM-backed resource loading (GitHub, GitLab, Harness Code, etc.)
│
└── examples/
    ├── zts/                  Full-featured server example (config, Docker, K8s manifests)
    ├── basic/                Minimal server with hardcoded verifiers
    ├── webhook_server/       Sample external policy webhook + custom webhook verifier
    └── monitoring/           Local Prometheus + Grafana stack
```

## How to Use

### Quick Start

```bash
# Prerequisites: Go 1.24+
git clone <repo-url> && cd zero-trust-service

mkdir -p /tmp/zts/audits
export ZTS_AUDIT_DIR=/tmp/zts/audits   # required on macOS; default /var/log/zts needs root
make run-example-zts                   # API :4210, admin :8898

# Delegate task
curl -s -X POST http://localhost:4210/api/verify \
  -H "Content-Type: application/json" \
  -d '{"taskPackage":{"delegateTaskId":"t1","accountId":"myAcct","data":{"taskType":"SHELL_SCRIPT_TASK_NG"}}}'

# GitOps agent task (same /api/verify endpoint, same taskPackage structure with agentId)
curl -s -X POST http://localhost:4210/api/verify \
  -H "Content-Type: application/json" \
  -d '{"taskPackage":{"taskId":"t1","accountId":"myAcct","gitOpsAgentId":"local-agent","data":{"taskType":"GITOPS_APP_SYNC"}}}'
```

For a more elaborate server example (Docker, K8s, config, env vars), see [`examples/zts/`](./examples/zts/).

### Using ZTS as a Library

At its simplest, you create a server with a verify handler:

```go
package main

import "github.com/harness/zero-trust-service"

func main() {
    srv := zts.NewServer(
        zts.WithPort(4210),
        zts.WithVerifyHandler(handler),
    )
    srv.ListenAndServe()
}
```

The handler is built by composing verifiers into a chain. Custom webhook verifiers let you add arbitrary policy logic without modifying ZTS — see [`examples/webhook_server/`](./examples/webhook_server/).

```go
chain := verifier.Chain(validatorA, validatorB, validatorC)
handler := verifier.ToHandler(chain)
```

### Writing a Custom Verifier

Implement the `verifier.Interface` or create one from a verify function:

```go
v := verifier.From(func(ctx context.Context, req types.VerifyRequest) error {
    // return nil → allow, return error → deny
    return nil
})
```

See [`examples/webhook_server/`](./examples/webhook_server/) for a complete webhook verifier example.

### Pluggable Backends

| Concern | Interface | Provided Implementations |
|---------|-----------|--------------------------|
| Authorization | `verifier.Interface` | `require_account`, `task_denylist`, `shellscript`, `image_allowlist`, `step_lookup`, `webhook` |
| Audit | `audit.Writer` | `audit/file` (local filesystem) |
| Metrics | `metrics.Emitter` | `metrics/prometheus`, `metrics.NewNoop()` |
| Template resolution | `resolver.ResourceLoader`, `resolver.TemplateStore` | `resolver/scm` (go-scm for GitHub, GitLab, etc.) |

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/verify` | POST | Authorize a delegate or GitOps agent task (unified `taskPackage`) |
| `/api/output` | POST | Receive task output from the delegate or GitOps agent |

Admin endpoints (metrics, healthz, audit queries) are hosted separately by the application — see [`examples/zts/`](./examples/zts/) for the reference setup.

## Project Structure

| Directory | README | Description |
|-----------|--------|-------------|
| [`verifier/`](./verifier/) | [README](./verifier/README.md) | Core `Interface`, chain, tracker, instrumentation |
| [`middleware/`](./middleware/) | [README](./middleware/README.md) | Verify/output middleware constructors (Logging, Metrics, Audit, MissingMetadata) |
| [`metrics/`](./metrics/) | [README](./metrics/README.md) | Metrics Emitter interface + Prometheus implementation |
| [`resolver/`](./resolver/) | [README](./resolver/README.md) | Pipeline template resolver |
| [`audit/`](./audit/) | [README](./audit/README.md) | Audit logging interface + file-based implementation |
| [`examples/zts/`](./examples/zts/) | [README](./examples/zts/README.md) | Full-featured ZTS server example (config, Docker, K8s) |
| [`examples/basic/`](./examples/basic/) | — | Minimal server with hardcoded verifiers |
| [`examples/webhook_server/`](./examples/webhook_server/) | — | Sample webhook policy server + custom verifier |
| [`examples/monitoring/`](./examples/monitoring/) | — | Local Prometheus + Grafana dashboard |

## Make Targets

| Target | Description |
|--------|-------------|
| `make run-example-zts` | Run the full-featured example locally |
| `make run-example-basic` | Run the minimal example |
| `make run-example-webhook-server` | Run the sample webhook policy server |
| `make test` | Run all tests |
| `make monitoring-up` | Start local Prometheus + Grafana |
| `make monitoring-down` | Stop monitoring stack |

For the full deployment guide (Docker, K8s, delegate configuration), see [`examples/zts/README.md`](examples/zts/README.md).

For details on each package, see the README inside that directory.

## Contributing

Contributions are welcome. Before your contribution can be accepted, you must sign the Harness Contributor License Agreement (CLA), which is handled automatically via GitHub the first time you open a pull request.

## License

Zero-Trust Service is licensed under the Apache License, Version 2.0. See [LICENSE.md](./LICENSE.md) for the full license text and [NOTICE.md](./NOTICE.md) for copyright and attribution notices.
