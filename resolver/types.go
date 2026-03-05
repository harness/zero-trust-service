// Package resolver provides the pipeline YAML template resolution engine.
// SCM-backed implementations live in resolver/scm.
package resolver

// Scope represents the Harness entity scope.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeOrg     Scope = "org"
	ScopeAccount Scope = "account"
)

// TemplateRef uniquely identifies a template in the Harness hierarchy.
type TemplateRef struct {
	Identifier        string
	VersionLabel      string
	Scope             Scope
	AccountIdentifier string
	OrgIdentifier     string
	ProjectIdentifier string
	GitBranch         string
}

// FileRef identifies a file in a git repository.
type FileRef struct {
	Repo         string
	Path         string
	Ref          string
	ConnectorRef string
}

// TemplateEntity represents a fetched template with its metadata and YAML content.
type TemplateEntity struct {
	Identifier        string
	VersionLabel      string
	Type              string
	YAML              string
	Scope             Scope
	OrgIdentifier     string
	ProjectIdentifier string
	AccountIdentifier string
}

// ResolvedPipeline is the output of the full pipeline resolution process.
type ResolvedPipeline struct {
	OriginalYAML  string
	ResolvedYAML  string
	TemplatesUsed []TemplateRef
}
