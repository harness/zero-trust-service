# audit

The audit package provides a **persistence-agnostic interface for recording ZTS authorization events**. Every time the Delegate asks ZTS to verify a task (or sends task output back), an audit record can be written — capturing what was requested, what decision was made, which validators ran, and the full raw payload for forensic review.

The package itself only defines the `Writer` interface and the record types. Actual storage is handled by implementations in subpackages (see `audit/file/` for the provided file-based backend). You can swap in any backend — database, object store, external service — by implementing `Writer`.

## Folder Structure

```
audit/
├── audit.go       Writer interface, AuditRecord interface, Record, OutputRecord
└── file/          File-based audit implementation (see file/README.md)
```

## How to Use

### The Writer Interface

```go
type Writer interface {
    WriteEvent(kind string, record AuditRecord, rawPayload json.RawMessage)
}
```

- **`kind`** — the event type being recorded (`"verify"` or `"output"`).
- **`record`** — structured metadata (implements `AuditRecord`).
- **`rawPayload`** — the full JSON body of the original request, stored as-is for later inspection.

### Record Types

| Type | Event Kind | Fields |
|------|-----------|--------|
| `Record` | `"verify"` | Task ID, account, task type, allowed/denied, failed validator, reason, error, duration, validators run |
| `OutputRecord` | `"output"` | Task ID, account, task type name, response code |

Both implement `AuditRecord`, which provides:

```go
type AuditRecord interface {
    AuditID() string    // unique identifier
    AuditDate() string  // "YYYY-MM-DD" in UTC, used for date-partitioned storage
}
```

### Implementing a Custom Backend

To write audit records to your own storage:

```go
type MyWriter struct { /* DB connection, client, etc. */ }

func (w *MyWriter) WriteEvent(kind string, record audit.AuditRecord, rawPayload json.RawMessage) {
    // persist record and/or rawPayload to your backend
}
```

Pass it to the verify/output middleware constructors:

```go
zts.WithVerifyMiddleware(verifymw.Audit(myWriter))
zts.WithOutputMiddleware(outputmw.Audit(myWriter))
```

The middleware calls `WriteEvent` synchronously — the implementation decides how to optimize (spawn a goroutine, batch events, etc.).

The `Writer` interface intentionally has no lifecycle methods — the application manages the backend's connection lifecycle separately.

### Adding a New Event Kind

1. Define a struct that implements `AuditRecord`.
2. Call `writer.WriteEvent("your_kind", record, payload)` from the relevant handler.

The `file/` implementation handles new kinds automatically (creates a new `.jsonl` stream per kind).
