# Custom Validators

Custom validators allow you to extend ZTS with your own validation logic without modifying ZTS library code. They run **after** global and task-type validators.

---

## Webhook Validator

The webhook validator calls an external HTTP endpoint (customer-hosted policy service) with the full `DelegateTaskPackage` payload as a JSON POST body. The external service inspects the payload and returns a JSON response indicating whether the task should be authorized or blocked.

### Flow

```
Delegate → ZTS /api/verify
              │
              ├─ Global validators (require_account, ...)
              ├─ Task-type validators (shellscript, image_allowlist, ...)
              └─ Custom validators
                   └─ webhook → POST to external service
                                    │
                                    ├─ { "allowed": true }                   → pass
                                    └─ { "allowed": false, "reason": "..." } → block
```

### Request (sent to external service)

ZTS POSTs the full `VerifyRequest` (which mirrors `DelegateTaskPackage`) as JSON:

```json
{
  "delegateTaskId": "abc-DEL",
  "accountId": "kmpySmUISimoRrJL6NL73w",
  "delegateId": "xyz",
  "delegateInstanceId": "inst-1",
  "data": {
    "taskType": "SHELL_SCRIPT_TASK_NG",
    "parameters": [ ... ]
  },
  "ztsMetadata": {
    "accountId": "kmpySmUISimoRrJL6NL73w",
    "orgIdentifier": "production",
    "projectIdentifier": "infra",
    "stepFqn": "pipeline.stages.deploy.spec.execution.steps.shell1",
    "pipelineGitDetails": {
      "repoName": "my-repo",
      "branch": "main",
      "commitId": "a1b2c3d"
    },
    "executionDetails": {
      "pipelineExecutionId": "exec-123",
      "stageExecutionId": "stage-456",
      "stepExecutionId": "step-789"
    }
  }
}
```

### Expected Response

**Authorized:**

```json
{ "allowed": true }
```

**Blocked:**

```json
{ "allowed": false, "reason": "Shell scripts are blocked in org 'production' by policy" }
```

The `reason` field is optional but recommended — it is surfaced in the ZTS API response and audit logs so operators can understand why a task was blocked.

### Configuration

The webhook validator is registered as `"webhook"` in the validator registry. Pass the following config keys:

```yaml
# Example config structure (adapt to your application's config format)
type: webhook
enabled: true
config:
  name: "org-policy-service"
  url: "https://policy.example.com/zts/validate"
  timeout: "5s"
  headers:
    Authorization: "Bearer <token>"
  fail_open: false
  allowed_status_codes: [200, 202]
```

| Key | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| `name` | string | No | URL | Friendly name for logging and metrics |
| `url` | string | **Yes** | — | Endpoint to POST to |
| `timeout` | string | No | `5s` | HTTP timeout (Go duration format) |
| `headers` | map | No | — | Extra HTTP headers to send with each request |
| `fail_open` | bool | No | `false` | If `true`, allow the task when the webhook is unreachable or returns an error |
| `allowed_status_codes` | []int | No | `200-299` | HTTP status codes from the webhook treated as a successful response |

### Behaviour

- **Webhook responds with allowed status code** → parse JSON body, check `allowed` field
- **Webhook responds with non-allowed status code** → block (or allow if `fail_open: true`)
- **Webhook is unreachable / times out** → block (or allow if `fail_open: true`)

### Multiple Webhooks

You can chain multiple webhook validators. They execute in order and short-circuit on the first failure:

```yaml
- type: webhook
  enabled: true
  config:
    name: "security-team"
    url: "https://security.example.com/zts/validate"
    fail_open: false

- type: webhook
  enabled: true
  config:
    name: "compliance-audit"
    url: "https://compliance.example.com/zts/audit"
    fail_open: true    # audit-only, don't block on failure
```

### Example Webhook Server

An example webhook server is provided at [`examples/webhook_server/`](../../examples/webhook_server/). It demonstrates how to implement a custom policy endpoint.

```bash
# From the repo root
make run-example-webhook-server
```

---

## Adding New Custom Validator Types

To add a new custom validator type (beyond webhook):

1. Create a new file in `validators/custom/` (e.g., `my_validator.go`)
2. Implement a constructor with signature `func(cfg map[string]any) (verifier.Interface, error)`
3. Register it in `validators/registry.go`:
   ```go
   Register("my_validator", custom.MyValidator)
   ```
4. Add a config entry for it in your application's configuration:
   ```yaml
   - type: my_validator
     enabled: true
     config:
       key: "value"
   ```

No changes to ZTS library code are needed — just the registry entry and config.
