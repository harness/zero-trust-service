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

package pipeline

import "testing"

// cdStep is the logical FQN of the "cd" stage step; caaStep is the follower that joins on it.
const (
	cdStep  = "pipeline.stages.cd.spec.execution.steps.ShellScript_1"
	caaStep = "pipeline.stages.caa.spec.execution.steps.ShellScript_1"
)

// cdInstances are the 8 case31 fan-out stage-instance step FQNs (values sorted by key: env, infra, service).
func cdInstances() []string {
	base := ".spec.execution.steps.ShellScript_1"
	sufs := []string{
		"cd_e2_i1_s1", "cd_e2_i1_s2", "cd_e2_i2_s1", "cd_e2_i2_s2",
		"cd_env1_cs_s1", "cd_env1_cs_s2", "cd_env1_cds_s1", "cd_env1_cds_s2",
	}
	out := make([]string, len(sufs))
	for i, s := range sufs {
		out[i] = "pipeline.stages." + s + base
	}
	return out
}

// TestMultiDeployFanOut verifies the multi-service/env/infra fan-out: the follower joins on all 8 instances, each resolves to its logical node, and fan-in gates until all have run.
func TestMultiDeployFanOut(t *testing.T) {
	yaml := readFixture(t, "case31_multi_env_fanout.yaml")
	g, err := BuildGraph(yaml)
	if err != nil || g == nil {
		t.Fatalf("BuildGraph returned (%v, %v)", g, err)
	}

	t.Run("follower expands to all 8 instances", func(t *testing.T) {
		res := g.Resolve(caaStep)
		if len(res.Ancestors) != 1 {
			t.Fatalf("expected 1 ancestor, got %d: %+v", len(res.Ancestors), res.Ancestors)
		}
		a := res.Ancestors[0]
		if a.FQN != cdStep {
			t.Errorf("ancestor FQN = %q, want %q", a.FQN, cdStep)
		}
		if !a.Expands {
			t.Error("ancestor should expand")
		}
		if !sameSet(a.Instances, cdInstances()) {
			t.Errorf("Instances = %v,\nwant %v", a.Instances, cdInstances())
		}
	})

	t.Run("each instance step resolves to the logical node", func(t *testing.T) {
		for _, inst := range cdInstances() {
			if res := g.Resolve(inst); !res.Found || res.FQN != cdStep {
				t.Errorf("Resolve(%s) = {Found:%v FQN:%q}, want {true %s}", inst, res.Found, res.FQN, cdStep)
			}
		}
	})

	t.Run("fan-in gated until all 8 instances run", func(t *testing.T) {
		all := cdInstances()
		if v, _ := VerifyStepOrder(yaml, caaStep, nil, WithRan(runSet(all[:7]...))); v.Allowed {
			t.Fatalf("caa should be denied at 7/8: %+v", v)
		}
		if v, _ := VerifyStepOrder(yaml, caaStep, nil, WithRan(runSet(all...))); !v.Allowed {
			t.Fatalf("caa should be allowed at 8/8: %+v", v)
		}
	})
}

// TestMultiDeployRuntimeInfra: a runtime-input infra (`<+input>`) must degrade the stage to logical-FQN gating, never nil the whole graph.
func TestMultiDeployRuntimeInfra(t *testing.T) {
	yaml := readFixture(t, "case32_multi_env_runtime_infra.yaml")
	g, err := BuildGraph(yaml)
	if err != nil || g == nil {
		t.Fatalf("BuildGraph returned (%v, %v): a runtime-input infra must not fail the build", g, err)
	}

	t.Run("follower resolves with a non-enumerable ancestor", func(t *testing.T) {
		res := g.Resolve(caaStep)
		if !res.Found {
			t.Fatal("caa not found: an ordinary step must resolve even beside a non-enumerable stage")
		}
		if len(res.Ancestors) != 1 || res.Ancestors[0].FQN != cdStep {
			t.Fatalf("ancestors = %+v, want single logical %s", res.Ancestors, cdStep)
		}
		if !res.Ancestors[0].Expands || res.Ancestors[0].Instances != nil {
			t.Errorf("expected an expanding-but-non-enumerable ancestor, got %+v", res.Ancestors[0])
		}
	})

	t.Run("a value-named instance folds to the logical stage node", func(t *testing.T) {
		// The store only sees instance FQNs; each must resolve and fold to the logical stage the follower gates on.
		for _, inst := range cdInstances() {
			if res := g.Resolve(inst); !res.Found || res.FQN != cdStep {
				t.Errorf("Resolve(%s) = {Found:%v FQN:%q}, want {true %s}", inst, res.Found, res.FQN, cdStep)
			}
		}
	})

	t.Run("verdict surfaces the non-enumerable ancestor as a runtime input", func(t *testing.T) {
		// Non-enumerable ancestor is reported under RuntimeFanouts (logical FQN, no instances) — the signal a caller keys on to fail open.
		v, _ := VerifyStepOrder(yaml, caaStep, nil)
		if !sameSet(runtimeFanouts(v), []string{cdStep}) {
			t.Errorf("RuntimeFanouts = %v, want [%s]", runtimeFanouts(v), cdStep)
		}
		// A non-enumerable ancestor contributes only its logical FQN to the
		// combined Ancestors list (no concrete instances).
		if !sameSet(ancestorFQNs(v), []string{cdStep}) {
			t.Errorf("Ancestors = %v, want [%s] (logical FQN, no instances)", ancestorFQNs(v), cdStep)
		}
	})
}

// TestMultiDeployFanOut_EnumerableHasNoRuntimeInput: an enumerable fan-out (case31) gates on concrete instances and must NOT report any runtime-input ancestor.
func TestMultiDeployFanOut_EnumerableHasNoRuntimeInput(t *testing.T) {
	yaml := readFixture(t, "case31_multi_env_fanout.yaml")
	v, _ := VerifyStepOrder(yaml, caaStep, nil)
	if len(runtimeFanouts(v)) != 0 {
		t.Errorf("RuntimeFanouts = %v, want empty for an enumerable fan-out", runtimeFanouts(v))
	}
	// The fanned-out stage contributes its concrete instance FQNs to Ancestors.
	if !sameSet(ancestorFQNs(v), cdInstances()) {
		t.Errorf("Ancestors = %v, want all 8 instances", ancestorFQNs(v))
	}
}
