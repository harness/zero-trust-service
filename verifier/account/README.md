# verifier/account

An **account allowlist validator** that ensures only tasks from pre-approved Harness accounts are allowed through ZTS. If a task arrives with an `accountId` that is not on the list, it is immediately denied — no further validators in the chain run.

This is typically the first validator in the global chain, acting as a front gate before any task-type or custom policy logic.

## Folder Structure

```
account/
└── account.go    Validator implementation and init()-based registration
```

## How It Works

The validator is registered under the type name `require_account`. When built from config, it converts the `allowed_accounts` list into a map for O(1) lookups.

On each request:
1. Extracts `accountId` from the task package.
2. If missing — denies with `"account_id is missing in request"`.
3. If not in the allowlist — denies with `"account <id> is not in the allowlist"`.
4. Otherwise — passes (returns `nil`).

### YAML Config

```yaml
global:
  - type: require_account
    config:
      allowed_accounts:
        - "account-id-1"
        - "account-id-2"
```
