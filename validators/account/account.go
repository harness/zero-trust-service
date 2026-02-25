package account

import (
	"context"
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// allowlist rejects requests whose account_id is not in the configured list.
//
// Config example:
//
//	type: require_account
//	config:
//	  allowed_accounts:
//	    - "acc1"
//	    - "acc2"
type allowlist struct {
	allowed map[string]struct{}
}

// Allowlist creates an account allowlist validator from config.
func Allowlist(cfg map[string]any) (verifier.Interface, error) {
	raw, ok := cfg["allowed_accounts"]
	if !ok {
		return nil, fmt.Errorf("require_account: missing 'allowed_accounts' in config")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("require_account: 'allowed_accounts' must be a list")
	}

	allowed := make(map[string]struct{}, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("require_account: each account must be a string, got %T", v)
		}
		allowed[s] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("require_account: allowed_accounts list is empty")
	}

	return &allowlist{allowed: allowed}, nil
}

// Handle validates that the request's account_id is in the allowlist.
func (v *allowlist) Handle(_ context.Context, request types.VerifyRequest) error {
	accountID := request.ResolveAccountID()
	if accountID == "" {
		return fmt.Errorf("require_account: account_id is missing in request")
	}
	if _, ok := v.allowed[accountID]; !ok {
		return fmt.Errorf("require_account: account %q is not in the allowlist", accountID)
	}
	return nil
}
