# ZTS Example — Full Production Server

A production-ready Zero-Trust Service built on the ZTS library. This example wires up all the components (validators, resolver, audit, metrics) and provides Docker images, Kubernetes manifests, and environment-driven configuration.

## Running Locally

```bash
# From the repo root — set a writable audit dir first (required on macOS)
mkdir -p /tmp/zts/audits
export ZTS_AUDIT_DIR=/tmp/zts/audits
export ZTS_ALLOWED_ACCOUNTS='["your-account-id"]'   # must match gitops-agent account
make run-example-zts

# Or from this directory (same env vars)
make run
```

**Keep using `make run-example-zts`** for normal local dev. It loads [`config.yaml`](./config.yaml).

### Deny task types (delegate + GitOps)

Global **task type denylist** for delegate and GitOps verify requests. Register under `validators.global` (recommended) so one config list applies to every task.

The default [`config.yaml`](./config.yaml) already wires this up:

```yaml
validators:
  global:
    - type: task_denylist
      enabled: ${ZTS_TASK_DENYLIST_ENABLED:-false}
      config:
        denied_types: ${ZTS_DENIED_TASK_TYPES:-[]}
```

Example with explicit denied types:

```yaml
validators:
  global:
    - type: task_denylist
      enabled: true
      config:
        denied_types:
          - GITOPS_APP_SYNC
          - GITOPS_APP_DELETE
          - SHELL_SCRIPT_TASK_NG
```

**Environment (local dev):**

```bash
export ZTS_TASK_DENYLIST_ENABLED=true
export ZTS_DENIED_TASK_TYPES='["GITOPS_APP_SYNC"]'   # add more types as needed
make run-example-zts
```

**curl** (expect `"allowed":false`):

```bash
curl -s -X POST http://localhost:4210/api/verify \
  -H "Content-Type: application/json" \
  -d '{"taskPackage":{"taskId":"t1","accountId":"kmpySmUISimoRrJL6NL73w","gitOpsAgentId":"local","data":{"taskType":"GITOPS_APP_SYNC"}}}'
```

**gitops-agent:** `GITOPS_AGENT_ZTS_MODE=enforce`, restart agent, trigger sync.

See [`examples/verifiers/task_denylist.go`](../verifiers/task_denylist.go) for the verifier implementation.

The default audit directory (`/var/log/zts/audits`) is not writable without root on macOS. Always set `ZTS_AUDIT_DIR` to a local path before starting the server.

The API server starts on `:4210` (configurable via `ZTS_SERVER_PORT`) and the admin server on `:8898` (configurable via `ZTS_ADMIN_PORT`).

| Server | Port | Endpoints |
|--------|------|-----------|
| API | 4210 | `POST /api/verify`, `POST /api/output` |
| Admin | 8898 | `GET /metrics`, `GET /healthz`, `GET /api/audits/*` |

### GitOps agent verify (local smoke test)

After the server is up, a GitOps-style verify request should return `allowed: true` when the account is in `ZTS_ALLOWED_ACCOUNTS`:

```bash
curl -s -X POST http://localhost:4210/api/verify \
  -H "Content-Type: application/json" \
  -d '{"taskPackage":{"delegateTaskId":"t1","accountId":"kmpySmUISimoRrJL6NL73w","gitOpsAgentId":"local-agent","data":{"taskType":"GITOPS_APP_SYNC"}}}'
```

Point the gitops-agent at ZTS with `GITOPS_AGENT_ZTS_URL=http://localhost:4210/api/` and `GITOPS_AGENT_ZTS_MODE=shadow` (or `enforce`). See example verifiers under [`examples/verifiers/`](../verifiers/).

## Configuration

All settings live in [`config.yaml`](./config.yaml) with `${VAR:-default}` environment variable interpolation.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ZTS_SERVER_PORT` | `4210` | API server port |
| `ZTS_ADMIN_PORT` | `8898` | Admin/metrics server port |
| **Account validation** | | |
| `ZTS_REQUIRE_ACCOUNT_ENABLED` | `true` | Enable global account allowlist |
| `ZTS_ALLOWED_ACCOUNTS` | `["kmpySmUISimoRrJL6NL73w"]` | Allowed Harness account IDs |
| `ZTS_GITOPS_APP_SYNC_ACCOUNT_CHECK` | `true` | Per-task account check for `GITOPS_APP_SYNC` |
| `ZTS_GITOPS_APP_DELETE_ACCOUNT_CHECK` | `true` | Per-task account check for `GITOPS_APP_DELETE` |
| **Task denylist** | | |
| `ZTS_TASK_DENYLIST_ENABLED` | `false` | Enable global `task_denylist` verifier |
| `ZTS_DENIED_TASK_TYPES` | `[]` | JSON array of task types to deny (delegate or GitOps) |
| **Image allowlist** | | |
| `ZTS_IMAGE_ALLOWLIST_ENABLED` | `true` | Enable container image validation |
| `ZTS_IMAGE_ALLOWED_PREFIXES` | `["harness/", "library/"]` | Allowed image prefixes |
| **Shell script** | | |
| `ZTS_SHELLSCRIPT_ENABLED` | `false` | Enable shell script validator |
| `ZTS_SHELLSCRIPT_ALLOWED_COMMANDS` | `["echo"]` | Allowed shell commands |
| **Pipeline resolver** | | |
| `ZTS_RESOLVER_ENABLED` | `false` | Enable pipeline template resolution |
| `ZTS_DEFAULT_PROVIDER` | `github-main` | Default SCM provider for templates |
| `ZTS_TEMPLATE_MAPPINGS_FILE` | `mappings.yaml` | Template mappings file path |
| **Step lookup** | | |
| `ZTS_STEP_LOOKUP_ENABLED` | `false` | Enable step lookup (requires resolver) |
| `ZTS_STEP_LOOKUP_LOG_FOUND` | `true` | Log found steps |
| `ZTS_STEP_LOOKUP_LOG_MISSING` | `true` | Log missing steps |
| **Webhook** | | |
| `ZTS_WEBHOOK_ENABLED` | `false` | Enable custom webhook validator |
| `ZTS_WEBHOOK_URL` | `http://localhost:5050/zts/validate` | Webhook endpoint |
| `ZTS_WEBHOOK_TIMEOUT` | `5s` | Webhook request timeout |
| `ZTS_WEBHOOK_FAIL_OPEN` | `false` | Allow tasks if webhook is unreachable |
| **Audit** | | |
| `ZTS_AUDIT_ENABLED` | `true` | Enable audit logging |
| `ZTS_AUDIT_DIR` | `/var/log/zts/audits` | Audit log directory |
| `ZTS_AUDIT_MAX_AGE_DAYS` | `30` | Days to retain audit files |
| **SCM tokens** (secrets) | | |
| `GITHUB_TOKEN` | — | GitHub personal access token |
| `GITLAB_TOKEN` | — | GitLab personal access token |
| `HARNESS_CODE_TOKEN` | — | Harness Code API key |

## Docker

### Build

```bash
# Default (uses Dockerfile)
make docker-build

# Platform-specific (uses multi-stage Dockerfile.linux.amd64)
GOOS=linux GOARCH=amd64 make docker-build

# With custom registry
ZTS_REGISTRY=us.gcr.io/my-project ZTS_TAG=v1.0 make docker-build
```

### Run

```bash
make docker-run
```

### Push

```bash
ZTS_REGISTRY=us.gcr.io/my-project ZTS_TAG=v1.0 make docker-build-push
```

## Kubernetes Deployment

Manifests are in [`deploy/`](./deploy/):

```
deploy/
├── namespace.yaml       # zero-trust-service namespace
├── deployment.yaml      # Pod spec with health probes and resource limits
├── service.yaml         # ClusterIP service on port 4210
├── secrets.yaml         # SCM tokens (fill in before applying)
├── values.yaml          # ConfigMap for env var overrides
└── mappings.yaml        # Template mappings ConfigMap
```

### Quick deploy

```bash
# Apply all manifests at once (after filling in secrets.yaml)
kubectl apply -f deploy/

# Or apply individually for more control:
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/secrets.yaml    # ← fill in tokens first
kubectl apply -f deploy/values.yaml
kubectl apply -f deploy/mappings.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```

### Delegate Configuration

Point the Harness Delegate at ZTS by setting:

```yaml
- name: ZERO_TRUST_SERVICE_ENABLED
  value: "true"
- name: ZERO_TRUST_SERVICE_URL
  value: "http://zero-trust-service.zero-trust-service.svc.cluster.local:4210"
```

### GitOps Agent Configuration

Point the GitOps agent at ZTS via Helm values:

```bash
helm upgrade argocd gitops-agent/gitops-helm \
  --values override.yaml \
  --namespace <agent-namespace> \
  --set agent.zts.url=http://zero-trust-service.zero-trust-service.svc.cluster.local:4210/api/ \
  --set agent.zts.mode=shadow
```

Modes: `shadow` (log only, never block — start here), `enforce` (block denied tasks), `disabled`.

After validating in shadow mode, switch to enforce:

```bash
helm upgrade argocd gitops-agent/gitops-helm \
  --set agent.zts.mode=enforce
kubectl rollout restart deployment gitops-agent -n <agent-namespace>
```

## Policy Samples

Deny destructive GitOps operations using env vars on the default config:

```bash
# Deny destructive GitOps operations (recommended for production):
export ZTS_TASK_DENYLIST_ENABLED=true
export ZTS_DENIED_TASK_TYPES='["GITOPS_APP_SYNC","GITOPS_APP_DELETE","GITOPS_CLUSTER_DELETE","GITOPS_REPO_DELETE","GITOPS_REPO_CREDS_CREATE"]'
make run-example-zts
```

For K8s deployments, set these same keys in the [`deploy/values.yaml`](./deploy/values.yaml) ConfigMap.

## Template Mappings

The [`mappings.yaml`](./mappings.yaml) file routes specific templates to different SCM providers/repos. See [`resolver/README.md`](../../resolver/README.md) for the full mapping spec.

## Monitoring

Start a local Prometheus + Grafana stack (from repo root):

```bash
make monitoring-up       # start
make monitoring-down     # stop
make monitoring-clean    # stop and remove volumes
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

See [`examples/monitoring/`](../monitoring/) for the dashboard and scrape configuration.
