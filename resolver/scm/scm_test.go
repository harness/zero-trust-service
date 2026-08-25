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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/driver/github"
)

func TestNewLoader(t *testing.T) {
	client := &scm.Client{}
	loader := NewLoader(client)
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
}

func TestNewMultiLoader(t *testing.T) {
	clients := map[string]*scm.Client{
		"github": {},
		"gitlab": {},
	}
	ml := NewMultiLoader(clients)
	if ml == nil {
		t.Fatal("NewMultiLoader returned nil")
	}
	if len(ml.loaders) != 2 {
		t.Errorf("expected 2 loaders, got %d", len(ml.loaders))
	}
}

func TestMultiLoader_Loader(t *testing.T) {
	ml := NewMultiLoader(map[string]*scm.Client{
		"github": {},
	})

	t.Run("existing provider", func(t *testing.T) {
		l, err := ml.Loader("github")
		if err != nil {
			t.Fatalf("Loader() error: %v", err)
		}
		if l == nil {
			t.Fatal("Loader() returned nil")
		}
	})

	t.Run("nonexistent provider", func(t *testing.T) {
		_, err := ml.Loader("nonexistent")
		if err == nil {
			t.Error("Loader() should return error for nonexistent provider")
		}
	})
}

func TestMultiLoader_Find_NoLoaders(t *testing.T) {
	ml := NewMultiLoader(map[string]*scm.Client{})
	_, err := ml.Find(context.TODO(), "repo", "path", "ref")
	if err != resolver.ErrNoLoader {
		t.Errorf("expected ErrNoLoader, got %v", err)
	}
}

func TestTemplateStore_ResolveFileLocation(t *testing.T) {
	cfg := resolver.ResolverConfig{
		SCM: resolver.SCMConfig{
			Providers: map[string]resolver.SCMProviderConfig{
				"github": {
					Owner:    "owner",
					Repo:     "repo",
					Branch:   "main",
					BasePath: ".harness",
				},
			},
		},
		Templates: resolver.TemplateStoreConfig{
			DefaultProvider: "github",
		},
	}
	ml := NewMultiLoader(map[string]*scm.Client{"github": {}})
	store := NewTemplateStore(ml, cfg, nil)

	t.Run("bare repo is qualified with owner", func(t *testing.T) {
		ref := resolver.TemplateRef{
			Identifier:   "my_template",
			VersionLabel: "v1",
			Scope:        resolver.ScopeAccount,
		}
		provider, repo, filePath, branch := store.resolveFileLocation(ref)
		if provider != "github" {
			t.Errorf("provider = %q, want github", provider)
		}
		if repo != "owner/repo" {
			t.Errorf("repo = %q, want owner/repo (owner should be prepended)", repo)
		}
		if filePath != ".harness/templates/my_template/v1.yaml" {
			t.Errorf("filePath = %q", filePath)
		}
		if branch != "main" {
			t.Errorf("branch = %q, want main", branch)
		}
	})

	t.Run("already qualified repo is left as-is", func(t *testing.T) {
		cfgQualified := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"github": {
						Owner:    "owner",
						Repo:     "other-owner/other-repo",
						Branch:   "main",
						BasePath: ".harness",
					},
				},
			},
			Templates: resolver.TemplateStoreConfig{DefaultProvider: "github"},
		}
		store := NewTemplateStore(ml, cfgQualified, nil)
		ref := resolver.TemplateRef{Identifier: "t", VersionLabel: "v1", Scope: resolver.ScopeAccount}
		_, repo, _, _ := store.resolveFileLocation(ref)
		if repo != "other-owner/other-repo" {
			t.Errorf("repo = %q, want other-owner/other-repo", repo)
		}
	})

	t.Run("no owner leaves repo bare", func(t *testing.T) {
		cfgNoOwner := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"harness": {Repo: "test", Branch: "main", BasePath: ".harness"},
				},
			},
			Templates: resolver.TemplateStoreConfig{DefaultProvider: "harness"},
		}
		store := NewTemplateStore(ml, cfgNoOwner, nil)
		ref := resolver.TemplateRef{Identifier: "t", VersionLabel: "v1", Scope: resolver.ScopeAccount}
		_, repo, _, _ := store.resolveFileLocation(ref)
		if repo != "test" {
			t.Errorf("repo = %q, want test (no owner to prepend)", repo)
		}
	})

	t.Run("org scoped", func(t *testing.T) {
		ref := resolver.TemplateRef{
			Identifier:    "org_template",
			VersionLabel:  "v1",
			Scope:         resolver.ScopeOrg,
			OrgIdentifier: "testorg",
		}
		_, _, filePath, _ := store.resolveFileLocation(ref)
		want := ".harness/orgs/testorg/templates/org_template/v1.yaml"
		if filePath != want {
			t.Errorf("filePath = %q, want %q", filePath, want)
		}
	})

	t.Run("project scoped", func(t *testing.T) {
		ref := resolver.TemplateRef{
			Identifier:        "proj_template",
			VersionLabel:      "v1",
			Scope:             resolver.ScopeProject,
			OrgIdentifier:     "testorg",
			ProjectIdentifier: "testproj",
		}
		_, _, filePath, _ := store.resolveFileLocation(ref)
		want := ".harness/orgs/testorg/projects/testproj/templates/proj_template/v1.yaml"
		if filePath != want {
			t.Errorf("filePath = %q, want %q", filePath, want)
		}
	})

	t.Run("custom mapping overrides provider", func(t *testing.T) {
		cfg := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"github": {
						Owner:    "owner",
						Repo:     "repo",
						Branch:   "main",
						BasePath: ".harness",
					},
					"gitlab": {
						Owner:    "gl-owner",
						Repo:     "gl-repo",
						Branch:   "develop",
						BasePath: ".harness",
					},
				},
			},
			Templates: resolver.TemplateStoreConfig{
				DefaultProvider: "github",
			},
		}
		mappings := map[string]resolver.TemplateMappingConfig{
			"custom": {Version: "v1", Provider: "gitlab"},
		}
		store := NewTemplateStore(ml, cfg, mappings)

		ref := resolver.TemplateRef{
			Identifier:   "custom",
			VersionLabel: "v1",
			Scope:        resolver.ScopeProject,
		}
		provider, repo, _, branch := store.resolveFileLocation(ref)
		if provider != "gitlab" {
			t.Errorf("provider = %q, want gitlab", provider)
		}
		if repo != "gl-owner/gl-repo" {
			t.Errorf("repo = %q, want gl-owner/gl-repo", repo)
		}
		if branch != "develop" {
			t.Errorf("branch = %q, want develop", branch)
		}
	})

	t.Run("mapping repo override is also qualified", func(t *testing.T) {
		cfg := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"github": {Owner: "owner", Repo: "default-repo", Branch: "main", BasePath: ".harness"},
				},
			},
			Templates: resolver.TemplateStoreConfig{
				DefaultProvider: "github",
			},
		}
		mappings := map[string]resolver.TemplateMappingConfig{
			"special": {Version: "v1", Repo: "custom-repo", Path: ".harness/special.yaml"},
		}
		store := NewTemplateStore(ml, cfg, mappings)
		ref := resolver.TemplateRef{Identifier: "special", VersionLabel: "v1", Scope: resolver.ScopeProject}
		_, repo, filePath, _ := store.resolveFileLocation(ref)
		if repo != "owner/custom-repo" {
			t.Errorf("repo = %q, want owner/custom-repo (mapping repo should be qualified with owner)", repo)
		}
		if filePath != ".harness/special.yaml" {
			t.Errorf("filePath = %q, want .harness/special.yaml", filePath)
		}
	})

	t.Run("harness code repo stays bare", func(t *testing.T) {
		cfg := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"harness-code": {
						Driver:   resolver.DriverHarness,
						Owner:    "acct/org/proj",
						Repo:     "test",
						Branch:   "main",
						BasePath: ".harness",
					},
				},
			},
			Templates: resolver.TemplateStoreConfig{DefaultProvider: "harness-code"},
		}
		store := NewTemplateStore(ml, cfg, nil)
		ref := resolver.TemplateRef{
			Identifier:        "runstp",
			VersionLabel:      "v1",
			Scope:             resolver.ScopeProject,
			OrgIdentifier:     "default",
			ProjectIdentifier: "AmitTest2",
		}
		provider, repo, filePath, branch := store.resolveFileLocation(ref)
		if provider != "harness-code" {
			t.Errorf("provider = %q, want harness-code", provider)
		}
		if repo != "test" {
			t.Errorf("repo = %q, want test (harness driver handles owner internally)", repo)
		}
		if filePath != ".harness/orgs/default/projects/AmitTest2/templates/runstp/v1.yaml" {
			t.Errorf("filePath = %q", filePath)
		}
		if branch != "main" {
			t.Errorf("branch = %q, want main", branch)
		}
	})

	t.Run("harness code mapping repo also stays bare", func(t *testing.T) {
		cfg := resolver.ResolverConfig{
			SCM: resolver.SCMConfig{
				Providers: map[string]resolver.SCMProviderConfig{
					"github":       {Driver: resolver.DriverGitHub, Owner: "gh-owner", Repo: "gh-repo", Branch: "main", BasePath: ".harness"},
					"harness-code": {Driver: resolver.DriverHarness, Owner: "acct/org/proj", Repo: "default-repo", Branch: "main", BasePath: ".harness"},
				},
			},
			Templates: resolver.TemplateStoreConfig{DefaultProvider: "github"},
		}
		mappings := map[string]resolver.TemplateMappingConfig{
			"runstp": {Provider: "harness-code", Repo: "test"},
		}
		store := NewTemplateStore(ml, cfg, mappings)
		ref := resolver.TemplateRef{
			Identifier:        "runstp",
			VersionLabel:      "v1",
			Scope:             resolver.ScopeProject,
			OrgIdentifier:     "default",
			ProjectIdentifier: "AmitTest2",
		}
		_, repo, _, _ := store.resolveFileLocation(ref)
		if repo != "test" {
			t.Errorf("repo = %q, want test (harness mapping repo should stay bare)", repo)
		}
	})
}

func TestMatchMapping(t *testing.T) {
	tests := []struct {
		name    string
		mapping resolver.TemplateMappingConfig
		scope   resolver.Scope
		version string
		want    bool
	}{
		{"exact match", resolver.TemplateMappingConfig{Scope: "project", Version: "v1"}, resolver.ScopeProject, "v1", true},
		{"wildcard version", resolver.TemplateMappingConfig{Scope: "project", Version: "*"}, resolver.ScopeProject, "v2", true},
		{"no scope filter", resolver.TemplateMappingConfig{}, resolver.ScopeOrg, "v1", true},
		{"scope mismatch", resolver.TemplateMappingConfig{Scope: "account"}, resolver.ScopeProject, "v1", false},
		{"version mismatch", resolver.TemplateMappingConfig{Version: "v1"}, resolver.ScopeProject, "v2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchMapping(tt.mapping, tt.scope, tt.version)
			if got != tt.want {
				t.Errorf("matchMapping() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScopeString(t *testing.T) {
	tests := []struct {
		scope resolver.Scope
		want  string
	}{
		{resolver.ScopeAccount, "account"},
		{resolver.ScopeOrg, "org"},
		{resolver.ScopeProject, "project"},
	}
	for _, tt := range tests {
		got := string(tt.scope)
		if got != tt.want {
			t.Errorf("string(%v) = %q, want %q", tt.scope, got, tt.want)
		}
	}
}

func TestQualifyRepo(t *testing.T) {
	cfg := resolver.ResolverConfig{
		SCM: resolver.SCMConfig{
			Providers: map[string]resolver.SCMProviderConfig{
				"github":       {Driver: resolver.DriverGitHub, Owner: "default-owner"},
				"gitlab":       {Driver: resolver.DriverGitLab, Owner: ""},
				"harness-code": {Driver: resolver.DriverHarness, Owner: "acct/org/proj"},
			},
		},
	}

	tests := []struct {
		provider string
		repo     string
		want     string
	}{
		{"github", "already/qualified", "already/qualified"},
		{"github", "unqualified", "default-owner/unqualified"},
		{"gitlab", "unqualified", "unqualified"},
		{"harness-code", "test", "test"},
		{"harness-code", "acct/org/proj/test", "acct/org/proj/test"},
	}

	for _, tt := range tests {
		got := cfg.QualifyRepo(tt.provider, tt.repo)
		if got != tt.want {
			t.Errorf("QualifyRepo(%q, %q) = %q, want %q", tt.provider, tt.repo, got, tt.want)
		}
	}
}

func TestParseTemplateYAML_FullMetadata(t *testing.T) {
	yaml := []byte(`
template:
  identifier: myTemplate
  name: My Template
  versionLabel: v2
  type: Step
`)
	ref := resolver.TemplateRef{Identifier: "fallback", VersionLabel: "v1", Scope: resolver.ScopeAccount}
	entity, err := parseTemplateYAML(yaml, ref, "acc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.Identifier != "myTemplate" {
		t.Errorf("Identifier = %q, want myTemplate", entity.Identifier)
	}
	if entity.VersionLabel != "v2" {
		t.Errorf("VersionLabel = %q, want v2", entity.VersionLabel)
	}
	if entity.Type != "Step" {
		t.Errorf("Type = %q, want Step", entity.Type)
	}
	if entity.AccountIdentifier != "acc1" {
		t.Errorf("AccountIdentifier = %q, want acc1", entity.AccountIdentifier)
	}
	// Raw YAML should be preserved verbatim.
	if entity.YAML != string(yaml) {
		t.Error("YAML field not preserved")
	}
}

func TestParseTemplateYAML_FallsBackToRef(t *testing.T) {
	// Template YAML with no identifier/versionLabel → ref values used.
	yaml := []byte(`template: {}`)
	ref := resolver.TemplateRef{Identifier: "ref-id", VersionLabel: "v1", Scope: resolver.ScopeOrg, OrgIdentifier: "testorg"}
	entity, err := parseTemplateYAML(yaml, ref, "acc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.Identifier != "ref-id" {
		t.Errorf("Identifier = %q, want ref-id", entity.Identifier)
	}
	if entity.VersionLabel != "v1" {
		t.Errorf("VersionLabel = %q, want v1", entity.VersionLabel)
	}
	if entity.OrgIdentifier != "testorg" {
		t.Errorf("OrgIdentifier = %q, want testorg", entity.OrgIdentifier)
	}
}

func TestParseTemplateYAML_InvalidYAML(t *testing.T) {
	_, err := parseTemplateYAML([]byte(":\t:"), resolver.TemplateRef{}, "acc1")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNewClient_UnknownDriver(t *testing.T) {
	_, err := NewClient(resolver.SCMProviderConfig{Driver: "unsupported"})
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestNewClient_KnownDrivers(t *testing.T) {
	// Azure requires non-empty org+project in azure.New(), so it is tested separately.
	drivers := []resolver.Driver{
		resolver.DriverGitHub,
		resolver.DriverGitLab,
		resolver.DriverBitbucket,
		resolver.DriverStash,
		resolver.DriverGitee,
	}
	for _, d := range drivers {
		_, err := NewClient(resolver.SCMProviderConfig{Driver: d})
		if err != nil {
			t.Errorf("NewClient(%q) unexpected error: %v", d, err)
		}
	}
}

func TestNewClient_HarnessDriver(t *testing.T) {
	_, err := NewClient(resolver.SCMProviderConfig{Driver: resolver.DriverHarness, Owner: "acc/org/proj"})
	if err != nil {
		t.Fatalf("NewClient(harness) unexpected error: %v", err)
	}
}

func TestNewClients_ErrorOnBadDriver(t *testing.T) {
	_, err := NewClients(resolver.SCMConfig{
		Providers: map[string]resolver.SCMProviderConfig{
			"bad": {Driver: "unknown"},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestNewClients_Empty(t *testing.T) {
	clients, err := NewClients(resolver.SCMConfig{Providers: map[string]resolver.SCMProviderConfig{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}

func TestParseHarnessOwner(t *testing.T) {
	tests := []struct {
		owner              string
		wantAcct, wantOrg, wantProj string
	}{
		{"acct/org/proj", "acct", "org", "proj"},
		{"6dPIY0B4S96mB_ivo57p9Q/default/AmitTest2", "6dPIY0B4S96mB_ivo57p9Q", "default", "AmitTest2"},
		{"acct/org", "acct", "org", ""},
		{"acct", "acct", "", ""},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		acct, org, proj := parseHarnessOwner(tt.owner)
		if acct != tt.wantAcct || org != tt.wantOrg || proj != tt.wantProj {
			t.Errorf("parseHarnessOwner(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.owner, acct, org, proj, tt.wantAcct, tt.wantOrg, tt.wantProj)
		}
	}
}

// githubStore builds a TemplateStore whose "github" provider points at srv.
func githubStore(t *testing.T, srv *httptest.Server) *TemplateStore {
	t.Helper()
	client, err := github.New(srv.URL)
	if err != nil {
		t.Fatalf("github.New: %v", err)
	}
	ml := NewMultiLoader(map[string]*scm.Client{"github": client})
	cfg := resolver.ResolverConfig{
		SCM: resolver.SCMConfig{
			Providers: map[string]resolver.SCMProviderConfig{
				"github": {Driver: resolver.DriverGitHub, Owner: "owner", Branch: "main"},
			},
		},
		Templates: resolver.TemplateStoreConfig{DefaultProvider: "github"},
	}
	return NewTemplateStore(ml, cfg, nil)
}

func TestGetTemplate_LoaderNotFound(t *testing.T) {
	ml := NewMultiLoader(map[string]*scm.Client{})
	cfg := resolver.ResolverConfig{Templates: resolver.TemplateStoreConfig{DefaultProvider: "missing"}}
	store := NewTemplateStore(ml, cfg, nil)

	_, err := store.GetTemplate(context.Background(), "acc1",
		resolver.TemplateRef{Identifier: "t1", VersionLabel: "v1", Scope: resolver.ScopeAccount})
	if err == nil {
		t.Fatal("expected error when loader is missing")
	}
}

func TestGetTemplate_Success(t *testing.T) {
	tmplYAML := "template:\n  identifier: myTpl\n  type: Step\n  versionLabel: v1\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"path":    "path/to/tpl.yaml",
			"content": base64.StdEncoding.EncodeToString([]byte(tmplYAML)),
		})
	}))
	defer srv.Close()

	store := githubStore(t, srv)
	entity, err := store.GetTemplate(context.Background(), "acc1",
		resolver.TemplateRef{Identifier: "myTpl", VersionLabel: "v1", Scope: resolver.ScopeAccount})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.Identifier != "myTpl" || entity.Type != "Step" {
		t.Errorf("unexpected entity: %+v", entity)
	}
}

func TestGetTemplate_FindError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := githubStore(t, srv)
	_, err := store.GetTemplate(context.Background(), "acc1",
		resolver.TemplateRef{Identifier: "missing", VersionLabel: "v1", Scope: resolver.ScopeAccount})
	if err == nil {
		t.Fatal("expected error for 404 from SCM")
	}
}
