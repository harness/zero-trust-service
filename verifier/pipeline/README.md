# verifier/pipeline

A **step lookup validator** that checks whether the step referenced by a delegate task actually exists in the resolved pipeline YAML. When the pipeline resolver is enabled, this validator searches the fully-expanded YAML tree for the task's `stepFQN` (fully-qualified name) and logs whether the step was found or missing.

By default, this validator **logs only** — it does not block tasks. It is designed for observability and auditing: you can see which tasks correspond to real pipeline steps and which arrive with step FQNs that don't match anything in the pipeline definition.

## Folder Structure

```
pipeline/
├── step_lookup.go   StepLookup validator — searches resolved YAML for a step FQN
└── yaml.go          Pipeline YAML parsing and FQN-based tree walking utilities
```

## How It Works

The validator is registered under the type name `step_lookup`. On each request:

1. Reads the resolved pipeline from context (set by the resolver middleware). If no resolved pipeline is available, passes silently.
2. Extracts `stepFQN` from the task's ZTS metadata. If absent, passes silently.
3. Walks the YAML tree following the dot-separated FQN segments (e.g. `pipeline.stages.build.spec.execution.steps.run1`), handling mapping nodes, sequence items, `step`/`stage`/`stepGroup` wrappers, and `parallel` blocks.
4. Logs the result (found with step type, or missing) — does not return an error.

### YAML Config

```yaml
global:
  - type: step_lookup
    config:
      log_found: true
      log_missing: true
```

### yaml.go Utilities

The `yaml.go` file provides general-purpose helpers for navigating Harness pipeline YAML trees:

| Function | Purpose |
|----------|---------|
| `ParsePipeline` | Parse YAML string into a `*yaml.Node` |
| `FindNodeByFQN` | Walk the tree by dot-separated FQN segments |
| `GetParentNode` | Walk to the parent of a given FQN |
| `GetNodeType` | Read the `type` key from a mapping node |
| `GetNodeScalar` | Read any scalar key from a mapping node |
| `GetNodeKeys` | List the keys of a mapping node |
| `GetNodeChild` | Get a child node by key |

These are used by `step_lookup` but are also available for any validator that needs to inspect pipeline YAML structure.
