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

package ordering

import (
	"testing"

	"github.com/harness/zero-trust-service/verifier/pipeline"
)

// tc29ResolvedYAML: multi-deploy "cd" stage (services s1,s2 x environments e2,env1,
// e2's infra is a runtime input) followed by "caa"; the engine fans out value-named instances.
const tc29ResolvedYAML = `
pipeline:
  identifier: zts_tc29_multi_env
  stages:
    - stage:
        identifier: cd
        type: Deployment
        spec:
          services:
            values:
              - serviceRef: s1
              - serviceRef: s2
          environments:
            values:
              - environmentRef: e2
                deployToAll: true
                infrastructureDefinitions: <+input>
              - environmentRef: env1
                infrastructureDefinitions:
                  - identifier: cs
                  - identifier: cds
          execution:
            steps:
              - step:
                  identifier: ShellScript_1
                  type: ShellScript
    - stage:
        identifier: caa
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: ShellScript_1
                  type: ShellScript
`

// verifyAndMark evaluates a step and, once allowed, records it under its logical FQN.
// It fails open on a step's OWN runtime fan-out (FanoutSelf) — an instance that is
// itself executing is legitimate — while still blocking when a *dependency* fans out
// unverifiably (FanoutAncestor). That Role distinction is the whole point: the same
// policy lets a runtime-parallelism step run yet keeps its followers fail-closed.
func verifyAndMark(t *testing.T, store *Store, exec, yaml, fqn string) (pipeline.Verdict, error) {
	t.Helper()
	v, err := pipeline.VerifyStepOrder(yaml, fqn, nil,
		pipeline.WithRan(func(f string) (bool, error) { return store.Ran(exec, f), nil }),
		pipeline.WithRuntimeFanoutPolicy(func(r *pipeline.RuntimeFanoutViolation) bool {
			return r.Role == pipeline.FanoutSelf
		}))
	if err == nil && v.Allowed {
		store.MarkRan(exec, fqn, v.FQN)
	}
	return v, err
}

// TestMultiDeployFlow_RuntimeInfra: value-named "cd" instances fold onto the logical
// "cd" node (numeric-only folding would miss value names and corrupt the "ShellScript_1"
// step id). Because "cd"'s infra is a runtime <+input>, its instance set is unknowable.
// A cd instance is itself a runtime fan-out (FanoutSelf), so it runs only under a
// self-fanout fail-open policy. Follower "caa" then still cannot be verified complete:
// cd is an ANCESTOR fan-out (FanoutAncestor), which that same self-only policy does not
// accept, so the SDK fail-closes on caa until a policy accepting ancestor fan-outs is set.
func TestMultiDeployFlow_RuntimeInfra(t *testing.T) {
	const exec = "xpUXM15FTZazuE-ehlbCvA"
	caa := "pipeline.stages.caa.spec.execution.steps.ShellScript_1"
	logicalCD := "pipeline.stages.cd.spec.execution.steps.ShellScript_1"

	instances := []string{
		"pipeline.stages.cd_e2_i1_s1.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_e2_i1_s2.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_e2_i2_s1.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_e2_i2_s2.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_env1_cs_s1.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_env1_cs_s2.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_env1_cds_s1.spec.execution.steps.ShellScript_1",
		"pipeline.stages.cd_env1_cds_s2.spec.execution.steps.ShellScript_1",
	}

	store := NewStore()

	if v, _ := verifyAndMark(t, store, exec, tc29ResolvedYAML, caa); v.Allowed {
		t.Fatalf("caa should be denied before cd runs: %+v", v)
	}

	// Each cd instance is an entry-stage step: allowed, and folds onto logical cd.
	for _, inst := range instances {
		v, err := verifyAndMark(t, store, exec, tc29ResolvedYAML, inst)
		if err != nil {
			t.Fatalf("cd instance %s not found: %v", inst, err)
		}
		if !v.Allowed {
			t.Fatalf("cd instance %s should be allowed (entry stage): %+v", inst, v)
		}
		if v.FQN != logicalCD {
			t.Fatalf("cd instance %s resolved to %q, want logical %q", inst, v.FQN, logicalCD)
		}
	}

	// Fail-closed by default: even with every cd instance run and folded onto logical
	// cd, the runtime-input fan-out is unverifiable, so caa is still denied — and the
	// violation is an ANCESTOR runtime fan-out (not a plain missing ancestor). The
	// self-only policy in verifyAndMark deliberately does not accept it.
	v, _ := verifyAndMark(t, store, exec, tc29ResolvedYAML, caa)
	if v.Allowed {
		t.Fatalf("caa should stay denied on an unverifiable runtime-input ancestor: %+v", v)
	}
	var rfv *pipeline.RuntimeFanoutViolation
	var missing []string
	for _, vi := range v.Violations {
		if r := pipeline.AsRuntimeFanoutViolation(vi); r != nil {
			rfv = r
		}
		if m := pipeline.AsAncestorDidNotRunViolation(vi); m != nil {
			missing = append(missing, m.Ancestors...)
		}
	}
	if rfv == nil {
		t.Fatalf("expected a RuntimeFanoutViolation, got %+v", v.Violations)
	}
	if rfv.FQN != logicalCD {
		t.Errorf("RuntimeFanout FQN = %v, want %s", rfv.FQN, logicalCD)
	}
	if rfv.Role != pipeline.FanoutAncestor {
		t.Errorf("RuntimeFanout Role = %v, want %v (cd is a dependency of caa)", rfv.Role, pipeline.FanoutAncestor)
	}
	if len(missing) != 0 {
		t.Errorf("Missing = %v, want empty (gated as runtime fan-out, not missing)", missing)
	}

	// A policy accepting the ancestor runtime fan-out lets caa through.
	vOpen, _ := pipeline.VerifyStepOrder(tc29ResolvedYAML, caa, nil,
		pipeline.WithRan(func(f string) (bool, error) { return store.Ran(exec, f), nil }),
		pipeline.WithRuntimeFanoutPolicy(func(*pipeline.RuntimeFanoutViolation) bool { return true }))
	if !vOpen.Allowed {
		t.Fatalf("caa should be allowed with a runtime-input fail-open policy: %+v", vOpen)
	}

	// Audit view keeps every cd instance individually, not folded. caa was never
	// allowed (fail-closed), so it is not recorded.
	if got := store.Instances(exec); len(got) != len(instances) {
		t.Errorf("audit view has %d instances, want %d", len(got), len(instances))
	}
}
