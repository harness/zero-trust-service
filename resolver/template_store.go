package resolver

import "context"

// TemplateStore abstracts fetching template entities by reference.
// Implement this to provide a custom template storage backend.
//
// See resolver/scm.TemplateStore for an SCM-backed implementation.
type TemplateStore interface {
	GetTemplate(ctx context.Context, accountID string, ref TemplateRef) (*TemplateEntity, error)
}
