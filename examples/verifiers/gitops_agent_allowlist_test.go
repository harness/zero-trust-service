package verifiers

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestNewGitopsAgentAllowlist_EmptyConfig(t *testing.T) {
	_, err := NewGitopsAgentAllowlist(GitopsAgentAllowlistConfig{})
	if err == nil {
		t.Fatal("expected error for empty allowed_agents")
	}
}

func TestNewGitopsAgentAllowlist_Valid(t *testing.T) {
	v, err := NewGitopsAgentAllowlist(GitopsAgentAllowlistConfig{AllowedAgents: []string{"agent-1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
}

func TestGitopsAgentAllowlist_Handle(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		req     types.VerifyRequest
		wantErr bool
	}{
		{"delegate task passes through", []string{"agent-1"}, types.VerifyRequest{}, false},
		{"allowed agent", []string{"prod-agent-1", "prod-agent-2"}, types.VerifyRequest{TaskPackage: &types.TaskPackage{GitOpsAgentID: "prod-agent-1"}}, false},
		{"blocked agent", []string{"prod-agent-1"}, types.VerifyRequest{TaskPackage: &types.TaskPackage{GitOpsAgentID: "unknown-agent"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := NewGitopsAgentAllowlist(GitopsAgentAllowlistConfig{AllowedAgents: tc.allowed})
			err := v.Handle(context.Background(), tc.req)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected pass, got %v", err)
			}
		})
	}
}
