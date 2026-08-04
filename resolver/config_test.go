package resolver

import (
	"testing"
)

func TestResolverConfig_Defaults(t *testing.T) {
	cfg := ResolverConfig{
		SCM: SCMConfig{
			Providers: map[string]SCMProviderConfig{
				"github": {},
			},
		},
	}
	cfg.Defaults()

	prov := cfg.SCM.Providers["github"]
	if prov.Branch != "main" {
		t.Errorf("expected Branch=main, got %s", prov.Branch)
	}
	if prov.BasePath != ".harness" {
		t.Errorf("expected BasePath=.harness, got %s", prov.BasePath)
	}
}

func TestResolverConfig_QualifyRepo(t *testing.T) {
	cfg := ResolverConfig{
		SCM: SCMConfig{
			Providers: map[string]SCMProviderConfig{
				"github":       {Driver: DriverGitHub, Owner: "owner"},
				"gitlab":       {Driver: DriverGitLab, Owner: ""},
				"harness-code": {Driver: DriverHarness, Owner: "acct/org/proj"},
			},
		},
	}

	tests := []struct {
		name     string
		provider string
		repo     string
		want     string
	}{
		{"already qualified", "github", "owner/repo", "owner/repo"},
		{"unqualified with owner", "github", "repo", "owner/repo"},
		{"unqualified no owner", "gitlab", "repo", "repo"},
		{"different owner in repo", "github", "other-owner/repo", "other-owner/repo"},
		{"harness code stays bare", "harness-code", "test", "test"},
		{"harness code already qualified stays as-is", "harness-code", "acct/org/proj/test", "acct/org/proj/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.QualifyRepo(tt.provider, tt.repo)
			if got != tt.want {
				t.Errorf("QualifyRepo(%q, %q) = %q, want %q", tt.provider, tt.repo, got, tt.want)
			}
		})
	}
}

func TestSCMProviderConfig_ResolveToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		envKey   string
		envValue string
		want     string
		setupEnv bool
	}{
		{
			name:  "literal token",
			token: "ghp_literal_token",
			want:  "ghp_literal_token",
		},
		{
			name:     "env var with dollar syntax",
			token:    "$TEST_TOKEN",
			envKey:   "TEST_TOKEN",
			envValue: "test_value_123",
			want:     "test_value_123",
			setupEnv: true,
		},
		{
			name:     "env var with brace syntax",
			token:    "${TEST_TOKEN_BRACE}",
			envKey:   "TEST_TOKEN_BRACE",
			envValue: "brace_value_456",
			want:     "brace_value_456",
			setupEnv: true,
		},
		{
			name:  "env var not set returns empty",
			token: "$NONEXISTENT_VAR",
			want:  "",
		},
		{
			name:  "empty token",
			token: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv {
				t.Setenv(tt.envKey, tt.envValue)
			}

			cfg := SCMProviderConfig{Token: tt.token}
			got := cfg.ResolveToken()
			if got != tt.want {
				t.Errorf("ResolveToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEnvVar(t *testing.T) {
	t.Setenv("TEST_VAR", "test_value")

	tests := []struct {
		name string
		val  string
		want string
	}{
		{"literal value", "literal", "literal"},
		{"dollar syntax", "$TEST_VAR", "test_value"},
		{"brace syntax", "${TEST_VAR}", "test_value"},
		{"nonexistent var", "$NONEXISTENT", ""},
		{"empty string", "", ""},
		{"single dollar", "$", "$"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnvVar(tt.val)
			if got != tt.want {
				t.Errorf("resolveEnvVar(%q) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}
