# verifier/tasktype

**Task-type-specific validators** that apply only when the delegate task matches a particular type. These are wired into the `by_task_type` section of the validator config and run through the `Dispatcher` — if the task type doesn't match, these validators are skipped entirely.

This package contains two built-in validators:

- **`shellscript`** — parses shell scripts using a real Bash parser and allows only approved commands.
- **`image_allowlist`** — checks that all container images referenced in a task match an approved prefix list.

## Folder Structure

```
tasktype/
├── shellscript.go       Shell script command validator
└── image_allowlist.go   Container image prefix validator
```

## shellscript

Validates delegate tasks of type `SHELL_SCRIPT_TASK_NG`. It extracts script content from the task parameters, parses it with a full Bash parser ([mvdan.cc/sh](https://github.com/mvdan/sh)), walks the AST, and checks every command invocation against an allowlist. Dynamic command names (e.g. `$CMD arg`) are always denied.

### YAML Config

```yaml
by_task_type:
  SHELL_SCRIPT_TASK_NG:
    - type: shellscript
      config:
        bash:
          - echo
          - curl
          - grep
```

### Denial Examples

- `rm -rf /` → denied if `rm` is not in the list
- `$(get_cmd) arg` → denied (dynamic command name)
- Script type `powershell` → denied (only `bash` is currently supported)

## image_allowlist

Validates container image references found anywhere in the task parameters JSON. It recursively walks the parameters looking for image names inside `imageDetails.name` and `stepInfo.image` fields, then checks each against a prefix list.

### YAML Config

```yaml
by_task_type:
  CI_BUILD:
    - type: image_allowlist
      config:
        allowed_prefixes:
          - "harness/"
          - "library/"
          - "us.gcr.io/my-project/"
```

### Denial Examples

- Image `evil/miner:latest` → denied if no prefix matches
- Image `harness/ci-addon:1.0` → allowed (matches `harness/` prefix)
