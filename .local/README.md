# ZTS Local Dev Environment

Everything in `.local/` is for local development and testing — **do not commit to git**.

---

## 1. Start ZTS Service

```bash
make run-example-zts
```

ZTS runs on `http://localhost:4210`.

---

## 2. Custom Webhook Validator

ZTS supports customer-provided validators via the `custom` section in `config.yaml`. The **webhook** validator calls an external HTTP endpoint with the full `DelegateTaskPackage` payload and expects a pass/fail JSON response.

### How it works

1. ZTS receives a `/api/verify` request.
2. After running global and task-type validators, ZTS POSTs the full payload to the configured webhook URL.
3. The external service inspects the payload and returns a JSON response.

### Expected webhook response

**Authorized:**

```json
{ "status": "authorized" }
```

**Blocked:**

```json
{ "status": "unauthorized", "error": "Shell scripts are blocked in org 'production' by policy" }
```

The `error` field is optional but recommended — it is surfaced in the ZTS API response and audit logs.

### Config (`config.yaml`)

```yaml
custom:
  - type: webhook
    enabled: true
    config:
      name: "org-policy-service"
      url: "http://localhost:5050/zts/validate"
      timeout: "5s"
      headers:
        Authorization: "Bearer <token>"
      fail_open: false                    # block tasks if webhook is unreachable
      allowed_status_codes: [200, 202]    # HTTP codes treated as success (default: 200-299)
```

| Key | Description |
|-----|-------------|
| `name` | Friendly name for logging and metrics |
| `url` | Endpoint to POST to (required) |
| `timeout` | HTTP timeout (default: `5s`) |
| `headers` | Extra HTTP headers to send |
| `fail_open` | If `true`, allow the task when the webhook is unreachable (default: `false`) |
| `allowed_status_codes` | HTTP status codes treated as success (default: `200-299`) |

### Example Webhook Server

An example webhook server is provided at `examples/webhook_server/main.go`. It blocks shell script tasks from the `production` org:

```bash
make run-example-webhook-server
```

Runs on `http://localhost:5050`. ZTS calls it at `/zts/validate`.

---

## 3. Start Prometheus + Grafana (Metrics)

```bash
make monitoring-up
```

| Service    | URL                        |
|------------|----------------------------|
| Prometheus | http://localhost:9091       |
| Grafana    | http://localhost:3002       |

Grafana credentials: `admin` / `admin`

A pre-loaded dashboard (**ZTS Overview**) is available under Dashboards.

Other commands:

```bash
make monitoring-down      # stop containers
make monitoring-restart   # restart containers
make monitoring-clean     # stop and delete all data volumes
```

Data is persisted in named Docker volumes (`prometheus-data`, `grafana-data`) and survives `make monitoring-down`. Use `make monitoring-clean` to wipe data.

---

## 4. Test Verify Endpoint

**Authorized request:**

```bash
curl -s http://localhost:4210/api/verify \
  -H 'Content-Type: application/json' \
  -d '{
    "delegateTaskId": "test-001",
    "accountId": "kmpySmUISimoRrJL6NL73w",
    "data": {
      "taskType": "INITIALIZATION_PHASE",
      "parameters": [{"containerImage": "harness/ci-lite-engine:latest"}]
    },
    "ztsMetadata": {
      "accountId": "kmpySmUISimoRrJL6NL73w",
      "orgIdentifier": "default",
      "projectIdentifier": "MyProject"
    }
  }' | jq .
```

**Blocked request (image not in allowlist):**

```bash
curl -s http://localhost:4210/api/verify \
  -H 'Content-Type: application/json' \
  -d '{
    "delegateTaskId": "test-002",
    "accountId": "kmpySmUISimoRrJL6NL73w",
    "data": {
      "taskType": "INITIALIZATION_PHASE",
      "parameters": [{"containerImage": "evil/miner:latest"}]
    },
    "ztsMetadata": {
      "accountId": "kmpySmUISimoRrJL6NL73w",
      "orgIdentifier": "default",
      "projectIdentifier": "MyProject"
    }
  }' | jq .
```

**Blocked by custom webhook (production org):**

```bash
curl -s http://localhost:4210/api/verify \
  -H 'Content-Type: application/json' \
  -d '{
    "delegateTaskId": "test-003",
    "accountId": "kmpySmUISimoRrJL6NL73w",
    "data": {
      "taskType": "SHELL_SCRIPT_TASK_NG"
    },
    "ztsMetadata": {
      "accountId": "kmpySmUISimoRrJL6NL73w",
      "orgIdentifier": "production",
      "projectIdentifier": "Infra"
    }
  }' | jq .
```

---

## 5. Query Audits

Audits are stored locally at the path configured in `config.yaml` → `audit.dir`.

**List audits (from/to are epoch milliseconds, required):**

```bash
# Last 24 hours
FROM=$(( $(date +%s) * 1000 - 86400000 ))
TO=$(( $(date +%s) * 1000 ))

curl -s "http://localhost:4210/api/audits?from=${FROM}&to=${TO}" | jq .
```

**Filter by account:**

```bash
curl -s "http://localhost:4210/api/audits?from=${FROM}&to=${TO}&account_id=kmpySmUISimoRrJL6NL73w" | jq .
```

**Filter by result:**

```bash
curl -s "http://localhost:4210/api/audits?from=${FROM}&to=${TO}&result=unauthorized" | jq .
```

**Get full payload for an audit entry:**

```bash
curl -s "http://localhost:4210/api/audits/<audit-id>/payload" | jq .
```

---

## File Layout

```
.local/
└── README.md                           ← this file

examples/
├── monitoring/                         ← Prometheus + Grafana stack
│   ├── docker-compose.yml
│   ├── prometheus.yml                  ← scrape config (targets ZTS :4210)
│   └── grafana/
│       └── provisioning/
│           ├── datasources/
│           │   └── prometheus.yml
│           └── dashboards/
│               ├── provider.yml
│               └── json/
│                   └── zts-overview.json   ← pre-loaded Grafana dashboard
├── webhook_server/
│   └── main.go                         ← example webhook validator server
├── zts/
│   └── main.go                         ← runnable ZTS example
└── basic/
    └── main.go                         ← minimal example
```
