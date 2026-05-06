# resolver/scm

**SCM-backed implementations** of the resolver's `ResourceLoader` and `TemplateStore` interfaces. This package connects ZTS's pipeline template resolution engine to real Git repositories using [go-scm](https://github.com/drone/go-scm), supporting GitHub, GitLab, Harness Code, Bitbucket, Azure DevOps, Stash, and Gitee.

When the resolver needs to fetch a pipeline YAML file or resolve a template reference, the `scm` package handles the actual Git API calls — determining which provider to use, qualifying the repository name, building the file path, and parsing the response.

## Folder Structure

```
scm/
├── factory.go          Creates go-scm clients from provider config (one per SCM provider)
├── loader.go           Loader / MultiLoader — implements ResourceLoader via go-scm
├── template_store.go   TemplateStore — resolves templateRef → file path, fetches & parses
└── scm_test.go         Tests
```

## How It Works

### factory.go

`NewClient(cfg)` creates a single `go-scm` client for a provider. It selects the correct driver (GitHub, GitLab, Harness, etc.), sets up the base URL, and configures authentication (bearer token or Harness API key).

`NewClients(cfg)` creates a named map of clients — one per provider entry in the resolver config.

Supported drivers: `github`, `gitlab`, `harness`, `bitbucket`, `stash`, `azure`, `gitee`.

### loader.go

- **`Loader`** — wraps a single `go-scm` client and implements `resolver.ResourceLoader`. Its `Find(ctx, repo, path, ref)` method calls the SCM content API to fetch a file.
- **`MultiLoader`** — holds multiple named `Loader`s and routes requests to the correct one by provider name. Also provides `Loader(name)` to get a specific `ResourceLoader` for downstream wiring.

### template_store.go

`TemplateStore` implements `resolver.TemplateStore`. When `GetTemplate` is called with a `TemplateRef`:

1. **Checks template mappings** — if the template identifier has an explicit mapping entry, uses its overrides for provider, repo, branch, and/or path.
2. **Falls back to convention** — builds the file path from the template's scope:
   - Account: `<base>/templates/<id>/<version>.yaml`
   - Org: `<base>/orgs/<org>/templates/<id>/<version>.yaml`
   - Project: `<base>/orgs/<org>/projects/<project>/templates/<id>/<version>.yaml`
3. **Qualifies the repo name** — for non-Harness providers, prepends the owner (e.g. `myorg/myrepo`). Harness Code handles this internally via the driver.
4. **Fetches and parses** — loads the YAML via the `MultiLoader`, unmarshals it, and returns a `TemplateEntity`.

### Wiring

```go
clients, _ := scm.NewClients(resolverConfig.SCM)
multiLoader := scm.NewMultiLoader(clients)

templateStore := scm.NewTemplateStore(multiLoader, resolverConfig, mappings)
pipelineLoader, _ := multiLoader.Loader(defaultProvider)

engine := resolver.New(templateStore, pipelineLoader)
```
