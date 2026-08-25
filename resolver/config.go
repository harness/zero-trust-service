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

package resolver

import (
	"os"
	"strings"
)

const MaxTemplateDepth = 10

// Driver identifies the SCM provider type.
type Driver string

const (
	DriverGitHub    Driver = "github"
	DriverGitLab    Driver = "gitlab"
	DriverBitbucket Driver = "bitbucket"
	DriverStash     Driver = "stash"
	DriverAzure     Driver = "azure"
	DriverGitee     Driver = "gitee"
	DriverHarness   Driver = "harness"
)

type ResolverConfig struct {
	Enabled   bool                `yaml:"enabled"`
	OutputDir string              `yaml:"output_dir,omitempty"`
	SCM       SCMConfig           `yaml:"scm"`
	Templates TemplateStoreConfig `yaml:"templates"`
}

type SCMConfig struct {
	Providers map[string]SCMProviderConfig `yaml:"providers"`
}

type SCMProviderConfig struct {
	Driver Driver `yaml:"driver"`
	URL    string `yaml:"url"`
	Token  string `yaml:"token"`

	// Per-provider defaults for template/pipeline file resolution.
	Owner    string `yaml:"owner,omitempty"`
	Repo     string `yaml:"repo,omitempty"`
	Branch   string `yaml:"branch,omitempty"`
	BasePath string `yaml:"base_path,omitempty"`
}

type TemplateStoreConfig struct {
	DefaultProvider      string `yaml:"default_provider"`
	TemplateMappingsFile string `yaml:"template_mappings_file"`
}

type TemplateMappingConfig struct {
	Scope    string `yaml:"scope,omitempty"`
	Version  string `yaml:"version,omitempty"`
	Provider string `yaml:"provider,omitempty"`
	Repo     string `yaml:"repo,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Branch   string `yaml:"branch,omitempty"`
}

// ProviderDefaults returns the resolved defaults for a given provider,
// falling back to the default provider's settings.
func (c ResolverConfig) ProviderDefaults(providerName string) (owner, repo, branch, basePath string) {
	prov, ok := c.SCM.Providers[providerName]
	if ok {
		owner = prov.Owner
		repo = prov.Repo
		branch = prov.Branch
		basePath = prov.BasePath
	}
	if branch == "" {
		branch = "main"
	}
	if basePath == "" {
		basePath = ".harness"
	}
	return
}

// QualifyRepo prepends owner to a bare repo name for the given provider.
// For Harness Code providers, the repo is returned as-is because the go-scm
// driver handles the account/org/project prefix internally.
func (c ResolverConfig) QualifyRepo(providerName, repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	prov, ok := c.SCM.Providers[providerName]
	if !ok {
		return repo
	}
	if prov.Driver == DriverHarness {
		return repo
	}
	if prov.Owner != "" {
		return prov.Owner + "/" + repo
	}
	return repo
}

// ResolveToken returns the token, resolving "$VAR" / "${VAR}" via os.Getenv.
func (c SCMProviderConfig) ResolveToken() string {
	return resolveEnvVar(c.Token)
}

// resolveEnvVar resolves env-variable references in config values.
func resolveEnvVar(val string) string {
	if len(val) < 2 || val[0] != '$' {
		return val
	}
	name := val[1:]
	if len(name) >= 2 && name[0] == '{' && name[len(name)-1] == '}' {
		name = name[1 : len(name)-1]
	}
	defaultVal := ""
	if idx := strings.Index(name, ":-"); idx >= 0 {
		defaultVal = name[idx+2:]
		name = name[:idx]
	}
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}

func (c *ResolverConfig) Defaults() {
	for name, prov := range c.SCM.Providers {
		if prov.Branch == "" {
			prov.Branch = "main"
		}
		if prov.BasePath == "" {
			prov.BasePath = ".harness"
		}
		c.SCM.Providers[name] = prov
	}
}
