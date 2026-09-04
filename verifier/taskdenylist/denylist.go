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

package taskdenylist

import (
	"context"
	"fmt"

	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
)

// Config blocks task types listed in denied_types. Works for delegate and GitOps
// requests (uses types.VerifyRequest.ResolveTaskType()).
type Config struct {
	DeniedTypes []string `yaml:"denied_types"`
}

type denylist struct {
	denied map[string]struct{}
}

// New creates a global task-type denylist verifier.
func New(cfg Config) (verifier.Interface, error) {
	if len(cfg.DeniedTypes) == 0 {
		return nil, fmt.Errorf("task_denylist: denied_types must not be empty")
	}
	denied := make(map[string]struct{}, len(cfg.DeniedTypes))
	for _, t := range cfg.DeniedTypes {
		denied[t] = struct{}{}
	}
	return &denylist{denied: denied}, nil
}

func (v *denylist) Handle(_ context.Context, req types.VerifyRequest) error {
	taskType := req.ResolveTaskType()
	if taskType == "" {
		return nil
	}
	if _, ok := v.denied[taskType]; ok {
		return fmt.Errorf("task_denylist: task type %q is denied by policy", taskType)
	}
	return nil
}
