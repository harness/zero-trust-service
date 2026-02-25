package tasktype

import (
	"context"
	"fmt"
	"log"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// Dispatcher routes requests to the correct validators based on task_type.
// Customers add new task_type entries in config without modifying ZTS code.
type Dispatcher struct {
	chains map[string]verifier.Interface
}

// NewDispatcher builds a dispatcher from config, skipping disabled validators.
func NewDispatcher(byTaskType map[string][]config.ValidatorDef, build func(config.ValidatorDef) (verifier.Interface, error)) (*Dispatcher, int, error) {
	chains := make(map[string]verifier.Interface, len(byTaskType))
	total := 0

	for taskType, defs := range byTaskType {
		var validators []verifier.Interface
		for _, def := range defs {
			if !def.IsEnabled() {
				log.Printf("skipping disabled task_type %q validator %q", taskType, def.Type)
				continue
			}
			v, err := build(def)
			if err != nil {
				return nil, 0, fmt.Errorf("task_type %q, validator %q: %w", taskType, def.Type, err)
			}
			validators = append(validators, v)
		}
		if len(validators) > 0 {
			chains[taskType] = verifier.Chain(validators...)
			total += len(validators)
			log.Printf("registered %d validator(s) for task_type %q", len(validators), taskType)
		} else {
			log.Printf("no enabled validators for task_type %q, skipping", taskType)
		}
	}

	return &Dispatcher{chains: chains}, total, nil
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
