package scm

import (
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"github.com/drone/go-scm/scm"
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
	_, err := ml.Find(nil, "repo", "path", "ref")
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
