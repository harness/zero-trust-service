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

package account

import (
	"context"
	"fmt"

	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
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
