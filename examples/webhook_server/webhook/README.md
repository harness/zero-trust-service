# examples/webhook_server/webhook

A **webhook verifier** that forwards the entire ZTS verify request to an external HTTP endpoint for policy evaluation. This lets you add arbitrary authorization logic — written in any language, hosted anywhere — without modifying ZTS itself. The external service inspects the request and returns an allow/deny decision.

This is an example of how to build a custom verifier type. It is included in the `examples/` tree and is imported by the full ZTS example (`examples/zts/`) via a blank import.

## Folder Structure

```
custom/
├── webhook.go        Webhook validator implementation and init()-based registration
└── webhook_test.go   Tests (authorized, unauthorized, fail-open, fail-closed, config errors)
```

## How It Works

The validator is registered under the type name `webhook`. On each request:

1. Marshals the full `VerifyRequest` to JSON.
2. Sends it to the configured URL (default: `POST`).
3. Reads the response, which must be JSON: `{"allowed": true/false, "reason": "..."}`.
4. If `allowed` is `false`, denies the task with the provided reason.
5. If the external service is unreachable or returns a non-2xx status:
   - **`fail_open: true`** → task is allowed (the webhook is treated as advisory).
   - **`fail_open: false`** (default) → task is denied.

### YAML Config

```yaml
custom:
  - type: webhook
    config:
      name: "policy-service"
      url: "https://policy.example.com/zts/validate"
      method: POST
      timeout: 5s
      fail_open: false
      headers:
        Authorization: "Bearer <token>"
      allowed_status_codes: [200]
```

### Config Fields

| Field | Default | Description |
|-------|---------|-------------|
| `url` | (required) | Endpoint to call |
| `name` | value of `url` | Display name for logs and error messages |
| `method` | `POST` | HTTP method |
| `timeout` | `5s` | Request timeout |
| `fail_open` | `false` | Allow tasks when the webhook is unreachable |
| `headers` | `{}` | Extra HTTP headers to send |
| `allowed_status_codes` | `200-299` | HTTP status codes treated as success |

### Expected Response

```json
{
  "allowed": true,
  "reason": "optional explanation",
  "metadata": {}
}
```

If `allowed` is `false` and `reason` is empty, the denial message defaults to `"request denied by external validator"`.
