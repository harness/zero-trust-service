# audit/file

A **file-based audit backend** that stores ZTS authorization records on the local filesystem. Each event is split into two parts: a compact metadata line and the full raw JSON payload. Records are organized by date for natural rotation and cleanup.

This is the reference implementation of the `audit.Writer` interface. It is designed for single-node deployments, development, and environments where external audit infrastructure is not available.

The library is intentionally **write-only** — its job is to persist records to disk.

## Folder Structure

```
file/
├── types.go      Config
└── writer.go     Writer — appends metadata to .jsonl files, writes payload JSON files
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

- **Metadata files** are append-only JSONL (one JSON object per line). They contain the structured `audit.Record` or `audit.OutputRecord`.
- **Payload files** store the complete raw JSON body of the original request. They are keyed by the record's UUID and can be fetched individually.
- **Date directories** enable automatic cleanup — the writer's background goroutine removes directories older than `MaxAgeDays`.

## What It Does

- **`Writer`** implements `audit.Writer`. On each `WriteEvent` call, it appends a metadata line to the appropriate `.jsonl` file and writes the raw payload to a separate JSON file. File handles are kept open and reused within a day for performance.

The on-disk layout is a plain directory of JSONL and JSON files, so it can be consumed with any tooling (`jq`, log shippers, etc.).

## Lifecycle

The `Writer` has two lifecycle methods (not part of the `audit.Writer` interface — they are specific to this implementation):

- **`Start(ctx)`** — starts a background goroutine that runs daily cleanup of directories older than `MaxAgeDays`. Runs until the context is cancelled.
- **`Close()`** — flushes and closes all open file handles.

The application is responsible for calling both. See `examples/zts/` for a reference setup.
