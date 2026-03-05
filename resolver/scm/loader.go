package scm

import (
	"context"
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"github.com/drone/go-scm/scm"
)

// Loader implements resolver.ResourceLoader using a single go-scm client.
type Loader struct {
	client *scm.Client
}

// NewLoader creates a Loader backed by the given go-scm client.
func NewLoader(client *scm.Client) *Loader {
	return &Loader{client: client}
}

// Find fetches file content from the SCM repository.
func (l *Loader) Find(ctx context.Context, repo, path, ref string) ([]byte, error) {
	content, resp, err := l.client.Contents.Find(ctx, repo, path, ref)
	if err != nil {
		if resp != nil && resp.Status == 404 {
			return nil, fmt.Errorf("%w: %s/%s@%s", resolver.ErrNotFound, repo, path, ref)
		}
		return nil, fmt.Errorf("fetch %s/%s@%s: %w", repo, path, ref, err)
	}
	return content.Data, nil
}

// MultiLoader multiplexes across multiple named SCM providers.
type MultiLoader struct {
	loaders map[string]*Loader
}

// NewMultiLoader creates a MultiLoader from a map of named go-scm clients.
func NewMultiLoader(clients map[string]*scm.Client) *MultiLoader {
	loaders := make(map[string]*Loader, len(clients))
	for name, client := range clients {
		loaders[name] = NewLoader(client)
	}
	return &MultiLoader{loaders: loaders}
}

// Loader returns the ResourceLoader for the named provider.
func (m *MultiLoader) Loader(providerName string) (resolver.ResourceLoader, error) {
	loader, ok := m.loaders[providerName]
	if !ok {
		return nil, fmt.Errorf("%w: provider %q", resolver.ErrNoLoader, providerName)
	}
	return loader, nil
}

// Find delegates to the first available loader (convenience for single-provider setups).
func (m *MultiLoader) Find(ctx context.Context, repo, path, ref string) ([]byte, error) {
	if len(m.loaders) == 0 {
		return nil, resolver.ErrNoLoader
	}
	for _, loader := range m.loaders {
		return loader.Find(ctx, repo, path, ref)
	}
	return nil, resolver.ErrNoLoader
}
