# auditreader

Reads back and serves the audit records written by the `audit/file` library.

The `audit/file` library handles writing and reading ZTS authorization events
on disk. The HTTP handler is an application concern and lives here in the example.

## Contents

```
audit/file/
├── reader.go    Reader — scans the on-disk metadata/payload layout with filtering + pagination
│                ListRequest, ListResponse types
└── writer.go    Writer — persists ZTS authorization events to disk

auditreader/
└── handler.go   Handler — HTTP routes over audit/file.Reader
```

## On-Disk Layout

The `Reader` understands the layout produced by `audit/file.Writer`:

```
<audit_dir>/
  metadata/<YYYY-MM-DD>/<kind>.jsonl   One JSON line per event
  payloads/<YYYY-MM-DD>/<kind>/<id>.json   Full raw request payload
```

## HTTP Routes

`Handler.RegisterRoutes` mounts two endpoints:

- `GET /audits` — list records. Query params: `from` (required, epoch millis),
  `to`, `kind` (`verify` default, or `output`), `account_id`, `task_type`,
  `task_id`, `allowed` (`true`/`false`), `limit`, `offset`.
- `GET /audits/{id}/payload` — fetch the full raw payload for a record.

## Usage

```go
reader := auditreader.NewReader(cfg.Audit.Dir)
auditreader.NewHandler(reader).RegisterRoutes(adminMux)
```

See `examples/zts/main.go` for the full wiring alongside the `audit/file`
writer.
