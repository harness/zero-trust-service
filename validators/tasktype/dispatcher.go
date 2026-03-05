package tasktype

import (
	"context"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// Dispatcher routes requests to the correct validators based on task_type.
type Dispatcher struct {
	chains map[string]verifier.Interface
}

// NewDispatcher creates a dispatcher from pre-built validator chains keyed by task type.
func NewDispatcher(chains map[string]verifier.Interface) *Dispatcher {
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
