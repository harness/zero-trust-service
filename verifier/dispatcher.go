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

package verifier

import (
	"context"

	"github.com/harness/zero-trust-service/types"
)

// Dispatcher routes requests to the correct validators based on task_type.
type Dispatcher struct {
	chains map[string]Interface
}

// NewDispatcher creates a dispatcher from pre-built validator chains keyed by task type.
func NewDispatcher(chains map[string]Interface) *Dispatcher {
	return &Dispatcher{chains: chains}
}

// Handle dispatches the request to the validator chain for its task type.
func (d *Dispatcher) Handle(ctx context.Context, request types.VerifyRequest) error {
	taskType := request.ResolveTaskType()
	if taskType == "" {
		return nil
	}
	chain, ok := d.chains[taskType]
	if !ok {
		return nil
	}
	return chain.Handle(ctx, request)
}
