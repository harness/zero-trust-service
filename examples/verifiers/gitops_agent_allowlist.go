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

	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
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
