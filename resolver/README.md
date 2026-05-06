# resolver

The resolver package **recursively expands Harness template references in pipeline YAML**. Harness pipelines can reference shared templates at the Pipeline, Stage, and Step levels, and those templates can nest other templates. A delegate task only carries the Git coordinates of the top-level pipeline file — to know which steps actually exist, ZTS must fetch the file from Git, walk the YAML tree, resolve every `templateRef` / `templateInputs` pair, and produce a fully-expanded pipeline.

Once resolved, the flat YAML is available to downstream validators (like `step_lookup`) and is included in audit records for forensic review.

## Folder Structure

```
resolver/
├── types.go               DTOs: Scope, TemplateRef, FileRef, TemplateEntity, ResolvedPipeline
├── config.go              Config structs (providers, mappings)
├── errors.go              Sentinel errors (ErrNotFound, ErrMaxDepth, etc.)
├── resource_loader.go     ResourceLoader interface — fetch raw file content from a source
├── template_store.go      TemplateStore interface — fetch a template by reference
├── resolver.go            Core resolution engine
├── template.go            templateRef parsing, spec extraction, template merging
├── yaml_helpers.go        YAML node manipulation helpers
└── scm/                   SCM-backed implementations
    ├── factory.go          Creates go-scm clients (GitHub, GitLab, Harness Code, etc.)
    ├── loader.go           Loader / MultiLoader — fetches files via go-scm
    └── template_store.go   Maps TemplateRef → file path, fetches & parses template YAML
```

## How to Use

### Interfaces to Implement

| Interface | Method | Purpose |
|-----------|--------|---------|
| `ResourceLoader` | `Find(ctx, repo, path, ref) ([]byte, error)` | Fetch a raw file from a source (Git repo, filesystem, etc.) |
| `TemplateStore` | `GetTemplate(ctx, accountID, ref TemplateRef) (*TemplateEntity, error)` | Resolve a template reference to its parsed YAML content |

The `scm/` subpackage provides ready-made implementations of both using [go-scm](https://github.com/drone/go-scm), supporting GitHub, GitLab, Harness Code, Bitbucket, Azure DevOps, Stash, and Gitee.

### Resolution Flow

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

### Wiring (Using the SCM Subpackage)

```go
clients := scm.NewClients(providers)               // create go-scm clients per provider
multiLoader := scm.NewMultiLoader(clients)          // wrap into a MultiLoader

templateStore := scm.NewTemplateStore(multiLoader, cfg, mappings)  // TemplateStore
pipelineLoader := multiLoader.Loader(defaultProvider)              // ResourceLoader

resolver := resolver.New(templateStore, pipelineLoader)
```

See `examples/zts/` for a complete wiring example.

### How templateRef → File Path Resolution Works

When `TemplateStore.GetTemplate()` receives a `TemplateRef`, it determines which Git repo, file path, and branch to fetch:

1. **Check template mappings** — if the template identifier has an explicit entry, use its overrides (provider, repo, path, branch).

2. **Fall back to convention**:

```
<base_path>/templates/<id>/<version>.yaml                                    # account scope
<base_path>/orgs/<org>/templates/<id>/<version>.yaml                         # org scope
<base_path>/orgs/<org>/projects/<project>/templates/<id>/<version>.yaml      # project scope
```

### Adding a New SCM Provider

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

1. Add a case in [`scm/factory.go`](./scm/factory.go) for the [go-scm](https://github.com/drone/go-scm) driver
2. Add provider credentials to your application's config under the SCM providers section
3. Map template identifiers → providers in your template mappings file
