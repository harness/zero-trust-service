// Example: GitOps agent allowlist verifier
//
// This verifier checks that a task comes from an allowed agent identifier.
// For delegate tasks (GitOpsAgentID is empty) the verifier is a no-op.
//
// Usage in config.yaml:
//
//	validators:
//	  global:
//	    - type: gitops_agent_allowlist
//	      config:
//	        allowed_agents: ["prod-agent-1", "prod-agent-2"]
package verifiers

import (
	"context"
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

type GitopsAgentAllowlistConfig struct {
	AllowedAgents []string `yaml:"allowed_agents"`
}

type gitopsAgentAllowlist struct {
	allowed map[string]struct{}
}

func NewGitopsAgentAllowlist(cfg GitopsAgentAllowlistConfig) (verifier.Interface, error) {
	if len(cfg.AllowedAgents) == 0 {
		return nil, fmt.Errorf("gitops_agent_allowlist: allowed_agents must not be empty")
	}
	allowed := make(map[string]struct{}, len(cfg.AllowedAgents))
	for _, a := range cfg.AllowedAgents {
		allowed[a] = struct{}{}
	}
	return &gitopsAgentAllowlist{allowed: allowed}, nil
}

func (v *gitopsAgentAllowlist) Handle(_ context.Context, req types.VerifyRequest) error {
	agent := req.GitOpsAgentID()
	if agent == "" {
		return nil
	}
	if _, ok := v.allowed[agent]; !ok {
		return fmt.Errorf("gitops_agent_allowlist: agent %q is not in the allowlist", agent)
	}
	return nil
}
