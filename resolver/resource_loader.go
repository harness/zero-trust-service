package resolver

import "context"

// ResourceLoader abstracts fetching file content from a source control provider.
// Implement this to provide a custom file-fetching backend.
//
// See resolver/scm.Loader for an SCM-backed implementation.
type ResourceLoader interface {
	Find(ctx context.Context, repo, path, ref string) ([]byte, error)
}
