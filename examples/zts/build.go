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

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/harness/zero-trust-service/examples/zts/config"
	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
)

const (
	metricVerifiersRegistered = "zts_verifiers_registered"

	scopeGlobal   = "global"
	scopeTaskType = "task_type"
	scopeCustom   = "custom"

	keyScope = "scope"
)

// ResolveFunc creates a verifier by name from an arbitrary config value.
type ResolveFunc func(name string, cfg any) (verifier.Interface, error)

func buildVerifiers(defs []config.ValidatorDef, scope string, resolve ResolveFunc, wrap verifier.WrapFunc) ([]verifier.Interface, int, error) {
	var out []verifier.Interface
	count := 0
	for i, def := range defs {
		if !def.IsEnabled() {
			log.Printf("skipping disabled %s verifier %q", scope, def.Type)
			continue
		}
		v, err := resolve(def.Type, def.Config)
		if err != nil {
			return nil, 0, fmt.Errorf("%s verifier #%d (%s): %w", scope, i, def.Type, err)
		}
		if wrap != nil {
			v = wrap(def.Type, v)
		}
		out = append(out, v)
		count++
		log.Printf("registered %s verifier %q", scope, def.Type)
	}
	return out, count, nil
}

// BuildChain wires up the full verifier chain: global → task-type → custom.
// The resolve function creates verifiers by name; the wrap function adds instrumentation.
func BuildChain(cfg config.ValidatorsConfig, m metrics.Emitter, resolve ResolveFunc, wrap verifier.WrapFunc) (verifier.Interface, error) {
	var all []verifier.Interface

	global, globalCount, err := buildVerifiers(cfg.Global, "global", resolve, wrap)
	if err != nil {
		return nil, err
	}
	all = append(all, global...)

	taskTypeCount := 0
	if len(cfg.ByTaskType) > 0 {
		chains := make(map[string]verifier.Interface, len(cfg.ByTaskType))
		for tt, defs := range cfg.ByTaskType {
			built, count, err := buildVerifiers(defs, fmt.Sprintf("task_type[%s]", tt), resolve, wrap)
			if err != nil {
				return nil, err
			}
			if count > 0 {
				chains[tt] = verifier.Chain(built...)
				taskTypeCount += count
			}
		}
		if len(chains) > 0 {
			all = append(all, verifier.NewDispatcher(chains))
		}
	}

	custom, customCount, err := buildVerifiers(cfg.Custom, "custom", resolve, wrap)
	if err != nil {
		return nil, err
	}
	all = append(all, custom...)

	m.Gauge(metricVerifiersRegistered, float64(globalCount), metrics.Dim(keyScope, scopeGlobal))
	m.Gauge(metricVerifiersRegistered, float64(taskTypeCount), metrics.Dim(keyScope, scopeTaskType))
	m.Gauge(metricVerifiersRegistered, float64(customCount), metrics.Dim(keyScope, scopeCustom))

	if len(all) == 0 {
		return verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil }), nil
	}

	return verifier.Chain(all...), nil
}
