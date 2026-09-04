// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scm

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/harness/zero-trust-service/resolver"
	"gopkg.in/yaml.v3"
)

// TemplateStore implements resolver.TemplateStore by resolving template identifiers
// to file locations in SCM repositories using configuration-based mappings.
type TemplateStore struct {
	multiLoader *MultiLoader
	resolverCfg resolver.ResolverConfig
	mappings    map[string]resolver.TemplateMappingConfig
}

// NewTemplateStore creates a TemplateStore backed by the given MultiLoader, config,
// and template mappings (loaded from a mappings file by the caller).
func NewTemplateStore(loader *MultiLoader, cfg resolver.ResolverConfig, mappings map[string]resolver.TemplateMappingConfig) *TemplateStore {
	return &TemplateStore{
		multiLoader: loader,
		resolverCfg: cfg,
		mappings:    mappings,
	}
}

// GetTemplate fetches a template by reference from SCM.
func (s *TemplateStore) GetTemplate(ctx context.Context, accountID string, ref resolver.TemplateRef) (*resolver.TemplateEntity, error) {
	providerName, repo, filePath, branch := s.resolveFileLocation(ref)

	loader, err := s.multiLoader.Loader(providerName)
	if err != nil {
		return nil, fmt.Errorf("template %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}

	data, err := loader.Find(ctx, repo, filePath, branch)
	if err != nil {
		return nil, fmt.Errorf("%w: %s@%s (path=%s/%s@%s): %v",
			resolver.ErrTemplateNotFound, ref.Identifier, ref.VersionLabel, repo, filePath, branch, err)
	}

	entity, err := parseTemplateYAML(data, ref, accountID)
	if err != nil {
		return nil, fmt.Errorf("parse template %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}

	return entity, nil
}

// resolveFileLocation determines the SCM provider, repo, file path, and branch
// for a template reference. It first checks mappings (keyed by identifier),
// then falls back to the provider's defaults and convention-based paths.
//
// Convention (Harness Code layout):
//
//	account:  <base_path>/templates/<id>/<version>.yaml
//	org:      <base_path>/orgs/<org>/templates/<id>/<version>.yaml
//	project:  <base_path>/orgs/<org>/projects/<project>/templates/<id>/<version>.yaml
func (s *TemplateStore) resolveFileLocation(ref resolver.TemplateRef) (provider, repo, filePath, branch string) {
	templatesCfg := s.resolverCfg.Templates
	provider = templatesCfg.DefaultProvider

	// Check for a mapping override keyed by template identifier
	if mapping, ok := s.mappings[ref.Identifier]; ok {
		if matchMapping(mapping, ref.Scope, ref.VersionLabel) {
			if mapping.Provider != "" {
				provider = mapping.Provider
			}

			// Get provider defaults for the resolved provider
			_, defaultRepo, defaultBranch, basePath := s.resolverCfg.ProviderDefaults(provider)
			repo = defaultRepo
			branch = defaultBranch

			if mapping.Repo != "" {
				repo = mapping.Repo
			}
			if mapping.Branch != "" {
				branch = mapping.Branch
			}
			if ref.GitBranch != "" {
				branch = ref.GitBranch
			}
			// If mapping provides a direct path, use it and return
			if mapping.Path != "" {
				filePath = mapping.Path
				repo = s.qualifyRepo(provider, repo)
				return
			}
			// Fall through to convention-based path with these overrides
			_ = basePath
		}
	}

	// Use provider defaults
	_, defaultRepo, defaultBranch, basePath := s.resolverCfg.ProviderDefaults(provider)
	if repo == "" {
		repo = defaultRepo
	}
	if branch == "" {
		branch = defaultBranch
	}
	if ref.GitBranch != "" {
		branch = ref.GitBranch
	}

	// Qualify bare repo name with owner (skipped for Harness Code — the
	// go-scm driver handles the account/org/project prefix internally).
	repo = s.qualifyRepo(provider, repo)

	// Build convention-based file path
	version := ref.VersionLabel
	if version == "" {
		version = "stable"
	}

	switch ref.Scope {
	case resolver.ScopeAccount:
		filePath = path.Join(basePath, "templates", ref.Identifier, version+".yaml")
	case resolver.ScopeOrg:
		org := ref.OrgIdentifier
		if org == "" {
			org = "default"
		}
		filePath = path.Join(basePath, "orgs", org, "templates", ref.Identifier, version+".yaml")
	default:
		org := ref.OrgIdentifier
		if org == "" {
			org = "default"
		}
		proj := ref.ProjectIdentifier
		if proj == "" {
			proj = "default"
		}
		filePath = path.Join(basePath, "orgs", org, "projects", proj, "templates", ref.Identifier, version+".yaml")
	}
	return
}

// qualifyRepo prepends owner to a bare repo name for non-Harness providers.
// For Harness Code, the go-scm driver already incorporates the account/org/project
// into the API URL via buildHarnessURI, so the repo must remain bare.
func (s *TemplateStore) qualifyRepo(providerName, repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	prov, ok := s.resolverCfg.SCM.Providers[providerName]
	if !ok {
		return repo
	}
	// Harness Code driver handles owner (account/org/project) internally
	if prov.Driver == resolver.DriverHarness {
		return repo
	}
	if prov.Owner != "" {
		return prov.Owner + "/" + repo
	}
	return repo
}

// matchMapping checks whether a template mapping matches the given scope and version.
func matchMapping(m resolver.TemplateMappingConfig, scope resolver.Scope, version string) bool {
	if m.Scope != "" && !strings.EqualFold(m.Scope, string(scope)) {
		return false
	}
	if m.Version != "" && m.Version != "*" && m.Version != version {
		return false
	}
	return true
}

// parseTemplateYAML unmarshals raw YAML into a TemplateEntity, using the ref as fallback metadata.
func parseTemplateYAML(data []byte, ref resolver.TemplateRef, accountID string) (*resolver.TemplateEntity, error) {
	var wrapper struct {
		Template struct {
			Identifier   string `yaml:"identifier"`
			Name         string `yaml:"name"`
			VersionLabel string `yaml:"versionLabel"`
			Type         string `yaml:"type"`
		} `yaml:"template"`
	}

	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("%w: %v", resolver.ErrInvalidYAML, err)
	}

	entity := &resolver.TemplateEntity{
		Identifier:        ref.Identifier,
		VersionLabel:      ref.VersionLabel,
		Scope:             ref.Scope,
		OrgIdentifier:     ref.OrgIdentifier,
		ProjectIdentifier: ref.ProjectIdentifier,
		AccountIdentifier: accountID,
		YAML:              string(data),
	}

	if wrapper.Template.Identifier != "" {
		entity.Identifier = wrapper.Template.Identifier
	}
	if wrapper.Template.VersionLabel != "" {
		entity.VersionLabel = wrapper.Template.VersionLabel
	}
	if wrapper.Template.Type != "" {
		entity.Type = wrapper.Template.Type
	}

	return entity, nil
}
