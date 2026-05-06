# audit/file

A **file-based audit backend** that stores ZTS authorization records on the local filesystem. Each event is split into two parts: a compact metadata line (for fast scanning and filtering) and the full raw JSON payload (for detailed inspection). Records are organized by date for natural rotation and cleanup.

This is the reference implementation of the `audit.Writer` interface. It is designed for single-node deployments, development, and environments where external audit infrastructure is not available.

## Folder Structure

```
file/
├── types.go      Config, ListRequest, ListResponse
├── writer.go     Writer — appends metadata to .jsonl files, writes payload JSON files
├── reader.go     Reader — scans metadata files with filtering and pagination
└── handler.go    HTTP handlers for querying audit records (list + get payload)
```

### On-Disk Storage Layout

When the writer is running, it creates the following structure under the configured audit directory:

```
<audit_dir>/
  metadata/
    <YYYY-MM-DD>/
      verify.jsonl          One JSON line per verify event
      output.jsonl          One JSON line per output event
  payloads/
    <YYYY-MM-DD>/
      verify/
        <uuid>.json         Full request payload for a verify event
      output/
        <uuid>.json         Full request payload for an output event
```

- **Metadata files** are append-only JSONL (one JSON object per line). They contain the structured `audit.Record` or `audit.OutputRecord` — enough to filter and list events without loading full payloads.
- **Payload files** store the complete raw JSON body of the original request. They are keyed by the record's UUID and can be fetched individually.
- **Date directories** enable automatic cleanup — the writer's background goroutine removes directories older than `MaxAgeDays`.

## What It Does

- **`Writer`** implements `audit.Writer`. On each `WriteEvent` call, it appends a metadata line to the appropriate `.jsonl` file and writes the raw payload to a separate JSON file. File handles are kept open and reused within a day for performance.
- **`Reader`** scans metadata files across a date range, applies filters (account, task type, task ID, allowed/denied), and returns paginated results.
- **`Handler`** exposes the reader over HTTP with two routes:
  - `GET /audits` — list audit records with query parameters (`from`, `to`, `kind`, `account_id`, `task_type`, `task_id`, `allowed`, `limit`, `offset`).
  - `GET /audits/{id}/payload` — fetch the full raw payload for a specific audit record.

## Lifecycle

The `Writer` has two lifecycle methods (not part of the `audit.Writer` interface — they are specific to this implementation):

- **`Start(ctx)`** — starts a background goroutine that runs daily cleanup of directories older than `MaxAgeDays`. Runs until the context is cancelled.
- **`Close()`** — flushes and closes all open file handles.

The application is responsible for calling both. See `examples/zts/` for a reference setup.
