# resolver

Recursively expands Harness template references in pipeline YAML so that downstream validators (e.g. `step_lookup`) can inspect the fully-resolved pipeline.

## Why

Harness pipelines can reference shared templates at three levels — Pipeline, Stage, and Step — and those templates can nest other templates. A delegate task's `ztsMetadata` only carries the Git details of the **top-level** pipeline file. To know which steps actually exist in the pipeline, ZTS must fetch the file from Git, walk the YAML, resolve every `templateRef` / `templateInputs` pair, and produce a flat, fully-expanded YAML.

## Package Layout

```
resolver/
├── resource_loader.go     # Interface: fetch raw file content
├── template_store.go      # Interface: fetch a template by reference
├── types.go               # DTOs: Scope, TemplateRef, FileRef, TemplateEntity, ResolvedPipeline
├── errors.go              # Sentinel errors (ErrNotFound, ErrMaxDepth, ...)
├── config.go              # Config structs (providers, mappings)
├── resolver.go            # Core resolution engine
├── template.go            # templateRef parsing, spec extraction, template merging
├── yaml_helpers.go        # YAML node manipulation helpers
└── scm/                   # SCM-backed ResourceLoader + TemplateStore
    ├── factory.go          #   Creates go-scm clients (GitHub, GitLab, Harness Code, …)
    ├── loader.go           #   Loader / MultiLoader — fetches files via go-scm
    └── template_store.go   #   Maps TemplateRef → file path, fetches & parses template YAML
```

## Interfaces

| Interface | File | Method | Purpose |
|-----------|------|--------|---------|
| `ResourceLoader` | `resource_loader.go` | `Find(ctx, repo, path, ref)` | Fetch raw file bytes from a source |
| `TemplateStore` | `template_store.go` | `GetTemplate(ctx, accountID, ref)` | Resolve a `TemplateRef` → `TemplateEntity` |

## Resolution Engine

```
LoadAndResolvePipeline(accountID, orgID, projectID, fileRef)
  │
  ├─ ResourceLoader.Find(repo, path, ref)       fetch pipeline YAML from Git
  │
  ├─ parse YAML → walk tree looking for templateRef nodes
  │     │
  │     ├─ TemplateStore.GetTemplate(ref)        fetch template YAML
  │     ├─ ExtractTemplateSpec() + MergeTemplateInputs()
  │     └─ recurse (max depth = 10)
  │
  └─ return ResolvedPipeline { OriginalYAML, ResolvedYAML, TemplatesUsed }
```

## SCM Layer (`scm/`)

Implements `ResourceLoader` and `TemplateStore` using [go-scm](https://github.com/drone/go-scm).

| Component | What it does |
|-----------|-------------|
| `factory.go` → `NewClient(cfg)` | Creates a `go-scm` client for a single provider with auth |
| `loader.go` → `Loader` | Wraps a single `go-scm` client, implements `ResourceLoader.Find()` |
| `loader.go` → `MultiLoader` | Holds multiple named `Loader`s, routes requests to the correct provider |
| `template_store.go` → `TemplateStore` | Maps `TemplateRef` → file path (via mappings or convention), fetches & parses |

Supported SCM drivers: `github`, `gitlab`, `harness`, `bitbucket`, `stash`, `azure`, `gitee`.

### How `templateRef` → file path resolution works

When `TemplateStore.GetTemplate()` receives a `TemplateRef`, it determines which Git repo, file path, and branch to fetch:

1. **Check template mappings** — if the template identifier has an entry, use its overrides.

2. **Fall back to convention**:

```
<base_path>/templates/<id>/<version>.yaml                                    # account scope
<base_path>/orgs/<org>/templates/<id>/<version>.yaml                         # org scope
<base_path>/orgs/<org>/projects/<project>/templates/<id>/<version>.yaml      # project scope
```

### Wiring

The application is responsible for constructing the resolver with concrete implementations of `ResourceLoader` and `TemplateStore`. The `scm/` subpackage provides ready-made implementations:

```
NewClients(providers)                 create go-scm clients per provider
  └─ NewMultiLoader(clients)          wrap into a MultiLoader
        ├─ NewTemplateStore(multiLoader, cfg, mappings)   TemplateStore
        └─ multiLoader.Loader(defaultProvider)            ResourceLoader for pipeline files
resolver.New(store, loader)           core engine with both dependencies injected
```

See [`examples/zts/`](../examples/zts/) for a complete wiring example.

## Adding a New SCM Provider

1. Add a case in [`scm/factory.go`](scm/factory.go) for the `go-scm` driver
2. Add provider credentials to your application's config under the SCM providers section
3. Map template identifiers → providers in your template mappings file
