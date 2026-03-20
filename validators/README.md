# validators

Built-in validators and the registry/builder that wires them into the chain.

## Built-in Validators

| Type | Package | Scope | Description |
|------|---------|-------|-------------|
| `require_account` | `account/` | Global | Reject tasks from unlisted Harness accounts |
| `step_lookup` | `pipeline/` | Global | Log whether the step FQN exists in the resolved pipeline |
| `shellscript` | `tasktype/` | Task-type | Allow only approved shellscript types and commands |
| `image_allowlist` | `tasktype/` | Task-type | Allow only approved container image prefixes |
| `webhook` | `custom/` | Custom | Forward payload to an external policy endpoint |

## Execution Order

```
Global validators → Task-type dispatcher → Custom validators
```

Global validators run on **every** request. Task-type validators run only when the incoming `taskType` has a matching config entry. Custom validators run last.

## Registry

`registry.go` maps config type strings to factory functions:

```go
Register("my_validator", mypackage.MyFactory)
```

Factories have the signature `func(cfg map[string]any) (verifier.Interface, error)`.

## Adding a New Validator

1. Create a package (e.g. `validators/mycheck/`)
2. Implement a factory:
   ```go
   func MyCheck(cfg map[string]any) (verifier.Interface, error) {
       return verifier.From(func(ctx context.Context, req types.VerifyRequest) error {
           // validation logic — return error to block
           return nil
       }), nil
   }
   ```
3. Register in `registry.go`:
   ```go
   Register("my_check", mycheck.MyCheck)
   ```
4. Add a config entry in your application's configuration:
   ```yaml
   - type: my_check
     enabled: true
     config: {}
   ```

See [`custom/README.md`](./custom/README.md) for the webhook validator spec.
