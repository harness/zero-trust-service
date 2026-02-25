package tasktype

import (
	"context"
	"fmt"
	"strings"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// commandBlocklist scans the raw task parameters for blocked patterns.
//
// Config example:
//
//	type: command_blocklist
//	config:
//	  patterns:
//	    - "rm -rf /"
//	    - "rm -rf /*"
//	    - "mkfs."
type commandBlocklist struct {
	patterns []string
}

// CommandBlocklist creates a command blocklist validator from config.
func CommandBlocklist(cfg map[string]any) (verifier.Interface, error) {
	raw, ok := cfg["patterns"]
	if !ok {
		return nil, fmt.Errorf("command_blocklist: missing 'patterns' in config")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("command_blocklist: 'patterns' must be a list")
	}

	var patterns []string
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("command_blocklist: each pattern must be a string, got %T", v)
		}
		patterns = append(patterns, s)
	}
	if len(patterns) == 0 {
		return nil, fmt.Errorf("command_blocklist: patterns list is empty")
	}

	return &commandBlocklist{patterns: patterns}, nil
}

// Handle scans the raw parameters for any blocked pattern.
func (v *commandBlocklist) Handle(_ context.Context, request types.VerifyRequest) error {
	if request.TaskPackage == nil || request.TaskPackage.TaskDetails == nil || len(request.TaskPackage.TaskDetails.Parameters) == 0 {
		return nil
	}

	paramsStr := string(request.TaskPackage.TaskDetails.Parameters)
	for _, pattern := range v.patterns {
		if strings.Contains(paramsStr, pattern) {
			return fmt.Errorf("command_blocklist: blocked pattern %q found in task parameters", pattern)
		}
	}
	return nil
}
