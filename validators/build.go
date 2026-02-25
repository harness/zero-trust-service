package validators

import (
	"context"
	"fmt"
	"log"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/tasktype"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// buildValidators builds a list of instrumented validators from a slice of
// ValidatorDefs, skipping disabled ones. scope is used only for logging.
func buildValidators(defs []config.ValidatorDef, scope string, m *metrics.Metrics) ([]verifier.Interface, int, error) {
	var out []verifier.Interface
	count := 0
	for i, def := range defs {
		if !def.IsEnabled() {
			log.Printf("skipping disabled %s validator %q", scope, def.Type)
			continue
		}
		v, err := Build(def)
		if err != nil {
			return nil, 0, fmt.Errorf("%s validator #%d (%s): %w", scope, i, def.Type, err)
		}
		out = append(out, verifier.Instrumented(def.Type, v, m))
		count++
		log.Printf("registered %s validator %q", scope, def.Type)
	}
	return out, count, nil
}

// BuildFromConfig wires up the full verifier chain from config.
// Execution order: global → task-type dispatcher → custom.
func BuildFromConfig(cfg config.ValidatorsConfig, m *metrics.Metrics) (verifier.Interface, error) {
	var all []verifier.Interface

	// Global validators
	global, globalCount, err := buildValidators(cfg.Global, "global", m)
	if err != nil {
		return nil, err
	}
	all = append(all, global...)

	// Task-type dispatcher
	taskTypeCount := 0
	if len(cfg.ByTaskType) > 0 {
		buildFn := func(def config.ValidatorDef) (verifier.Interface, error) {
			v, err := Build(def)
			if err != nil {
				return nil, err
			}
			return verifier.Instrumented(def.Type, v, m), nil
		}
		dispatcher, count, err := tasktype.NewDispatcher(cfg.ByTaskType, buildFn)
		if err != nil {
			return nil, err
		}
		taskTypeCount = count
		if count > 0 {
			all = append(all, dispatcher)
		}
	}

	// Custom validators
	custom, customCount, err := buildValidators(cfg.Custom, "custom", m)
	if err != nil {
		return nil, err
	}
	all = append(all, custom...)

	// Set registered validator gauges
	m.ValidatorsRegistered.WithLabelValues(metrics.LabelScopeGlobal).Set(float64(globalCount))
	m.ValidatorsRegistered.WithLabelValues(metrics.LabelScopeTaskType).Set(float64(taskTypeCount))
	m.ValidatorsRegistered.WithLabelValues(metrics.LabelScopeCustom).Set(float64(customCount))

	if len(all) == 0 {
		return verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil }), nil
	}

	return verifier.Chain(all...), nil
}
