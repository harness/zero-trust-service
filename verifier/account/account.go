package account

import (
	"context"
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// Config holds the account allowlist configuration.
type Config struct {
	AllowedAccounts []string `yaml:"allowed_accounts"`
}

type accountVerifier struct {
	allowed map[string]struct{}
}

// New creates an account allowlist validator from typed config.
func New(cfg Config) (verifier.Interface, error) {
	if len(cfg.AllowedAccounts) == 0 {
		return nil, fmt.Errorf("require_account: allowed_accounts list is empty")
	}
	allowed := make(map[string]struct{}, len(cfg.AllowedAccounts))
	for _, a := range cfg.AllowedAccounts {
		allowed[a] = struct{}{}
	}
	return &accountVerifier{allowed: allowed}, nil
}

func (v *accountVerifier) Handle(_ context.Context, request types.VerifyRequest) error {
	accountID := request.ResolveAccountID()
	if accountID == "" {
		return fmt.Errorf("require_account: account_id is missing in request")
	}
	if _, ok := v.allowed[accountID]; !ok {
		return fmt.Errorf("require_account: account %q is not in the allowlist", accountID)
	}
	return nil
}
