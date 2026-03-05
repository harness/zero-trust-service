# audit

Audit logging for ZTS requests and task outputs.

`audit.go` defines the core `Writer` interface and record types. The `file/` subpackage provides a file-based reference implementation. Any backend (database, object store, external service) can be used by implementing the `Writer` interface.

## Package Layout

```
audit/
├── audit.go          # Writer interface, AuditRecord interface, Record, OutputRecord
└── file/
    ├── types.go      # Config, ListRequest, ListResponse
    ├── writer.go     # Writer — appends metadata to .jsonl files, writes payload JSON
    ├── reader.go     # Reader — scans metadata files, filters, paginates
    └── handler.go    # HTTP handlers for querying audit records
```

## Writer Interface

The `Writer` interface defines a single method:

```go
type Writer interface {
    WriteEvent(kind string, record AuditRecord, rawPayload json.RawMessage)
}
```

It does not own lifecycle methods — the application manages the writer's lifecycle.

## Event Kinds

| Constant | Description |
|----------|-------------|
| `EventVerify` (`"verify"`) | Delegate task authorization request |
| `EventOutput` (`"output"`) | Delegate task output response |

New event types can be added by defining a struct that implements `AuditRecord` and calling `writer.WriteEvent("kind", record, payload)`.

## `file/` Implementation

The `file/` subpackage provides a file-based `Writer`, `Reader`, and HTTP `Handler`.

### Storage Layout

```
<audit_dir>/
  metadata/
    <YYYY-MM-DD>/
      verify.jsonl        # one JSON line per verify request
      output.jsonl        # one JSON line per task output
  payloads/
    <YYYY-MM-DD>/
      verify/
        <uuid>.json       # full request payload
      output/
        <uuid>.json       # full output payload
```

### Lifecycle

`file.Writer` provides concrete lifecycle methods (not part of the `Writer` interface):

- `Start(ctx)` — starts a background goroutine for periodic cleanup of old audit files
- `Close()` — flushes pending writes and releases resources

The application is responsible for calling these.

### HTTP Handler

`file.Handler` implements `RouteRegistrar` and registers routes for querying stored audits. The application decides where to mount these (e.g. on an admin router).

See [`examples/zts/`](../examples/zts/) for a reference setup.
