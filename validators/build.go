package validators

import (
	"context"
	"fmt"
	"log"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/tasktype"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func buildValidators(defs []ValidatorDef, scope string, m *metrics.Metrics) ([]verifier.Interface, int, error) {
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

// BuildFromConfig wires up the full verifier chain: global → task-type → custom.
func BuildFromConfig(cfg ValidatorsConfig, m *metrics.Metrics) (verifier.Interface, error) {
	var all []verifier.Interface

	global, globalCount, err := buildValidators(cfg.Global, "global", m)
	if err != nil {
		return nil, err
	}
	all = append(all, global...)

	taskTypeCount := 0
	if len(cfg.ByTaskType) > 0 {
		chains := make(map[string]verifier.Interface, len(cfg.ByTaskType))
		for tt, defs := range cfg.ByTaskType {
			built, count, err := buildValidators(defs, fmt.Sprintf("task_type[%s]", tt), m)
			if err != nil {
				return nil, err
			}
			if count > 0 {
				chains[tt] = verifier.Chain(built...)
				taskTypeCount += count
			}
		}
		if len(chains) > 0 {
			all = append(all, tasktype.NewDispatcher(chains))
		}
	}

	custom, customCount, err := buildValidators(cfg.Custom, "custom", m)
	if err != nil {
		return nil, err
	}
	all = append(all, custom...)

	m.ValidatorsRegistered.Set(float64(globalCount), metrics.LabelScopeGlobal)
	m.ValidatorsRegistered.Set(float64(taskTypeCount), metrics.LabelScopeTaskType)
	m.ValidatorsRegistered.Set(float64(customCount), metrics.LabelScopeCustom)

	if len(all) == 0 {
		return verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil }), nil
	}

	return verifier.Chain(all...), nil
}
