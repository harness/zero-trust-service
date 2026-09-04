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
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// tc22WhenYAML: s1 -> s2, where s2 carries a `when` guard (stageStatus + JEXL condition).
const tc22WhenYAML = `
pipeline:
  identifier: zts_tc22_when_jexl
  variables:
    - name: run
      type: String
      value: "yes"
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: ShellScript
              - step:
                  identifier: s2
                  type: ShellScript
                  when:
                    stageStatus: Success
                    condition: "<+pipeline.variables.run> == \"yes\""
`

func stepReq(exec, fqn string) (context.Context, types.VerifyRequest) {
	ctx, h := verifier.WithPipelineHolder(context.Background())
	h.Set(&resolver.ResolvedPipeline{ResolvedYAML: tc22WhenYAML})
	return ctx, types.VerifyRequest{TaskPackage: &types.TaskPackage{
		ZTSMetadata: &types.ZTSMetadata{
			StepFQN:          fqn,
			ExecutionDetails: &types.ExecutionDetails{PipelineExecutionID: exec},
		},
	}}
}

// tc22SkipYAML: chain s1 -> s2 -> s3 where a false `when` on s2 skips it; s3 must
// still be allowed once s1 has run (the verifier steps over the skipped s2).
const tc22SkipYAML = `
pipeline:
  identifier: zts_tc22_skip
  variables:
    - name: run
      type: String
      value: "yes"
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: ShellScript
              - step:
                  identifier: s2
                  type: ShellScript
                  when:
                    condition: "<+pipeline.variables.run> == \"yes\""
              - step:
                  identifier: s3
                  type: ShellScript
`

func skipReq(exec, fqn string) (context.Context, types.VerifyRequest) {
	ctx, h := verifier.WithPipelineHolder(context.Background())
	h.Set(&resolver.ResolvedPipeline{ResolvedYAML: tc22SkipYAML})
	return ctx, types.VerifyRequest{TaskPackage: &types.TaskPackage{
		ZTSMetadata: &types.ZTSMetadata{
			StepFQN:          fqn,
			ExecutionDetails: &types.ExecutionDetails{PipelineExecutionID: exec},
		},
	}}
}

// demoRunEval stands in for a JEXL engine, evaluating `run == "yes"` from Vars.
var demoRunEval = ConditionEvaluatorFunc(func(_, _ string, vars map[string]string) (bool, bool) {
	return vars["pipeline.variables.run"] == "yes", true
})

// tc22StatusYAML mirrors the real tc22: a guarded s2 (stageStatus + condition)
// comes first, then a bare s1 that gates on s2.
const tc22StatusYAML = `
pipeline:
  identifier: zts_tc22_status
  stages:
    - stage:
        identifier: st1
        type: Custom
        when:
          pipelineStatus: Success
        spec:
          execution:
            steps:
              - step:
                  identifier: s2
                  type: ShellScript
                  when:
                    stageStatus: Success
                    condition: 1==2
              - step:
                  identifier: s1
                  type: ShellScript
`

func statusReq(exec, fqn string) (context.Context, types.VerifyRequest) {
	ctx, h := verifier.WithPipelineHolder(context.Background())
	h.Set(&resolver.ResolvedPipeline{ResolvedYAML: tc22StatusYAML})
	return ctx, types.VerifyRequest{TaskPackage: &types.TaskPackage{
		ZTSMetadata: &types.ZTSMetadata{
			StepFQN:          fqn,
			ExecutionDetails: &types.ExecutionDetails{PipelineExecutionID: exec},
		},
	}}
}

// TestOrderingVerifier_AssumeSuccessStatusGate pins the happy-path status
// assumption: with stageStatus assumed Success, a condition override alone
// resolves s2's guard. `1==2: true` -> s2 known-live, so follower s1 waits;
// `1==2: false` -> s2 known-skipped, so s1 is stepped over and allowed.
func TestOrderingVerifier_AssumeSuccessStatusGate(t *testing.T) {
	const exec = "tc22-assume-success"
	s1 := "pipeline.stages.st1.spec.execution.steps.s1"

	t.Run("condition true -> s2 known-live -> s1 waits", func(t *testing.T) {
		chk := NewConditionChecker(nil, nil)
		chk.Overrides = map[string]bool{"1==2": true}
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx, req := statusReq(exec, s1)
		if err := v.Handle(ctx, req); err == nil {
			t.Fatal("s1 must be denied while its known-live ancestor s2 has not run")
		}
	})

	t.Run("condition false -> s2 known-skipped -> s1 allowed", func(t *testing.T) {
		chk := NewConditionChecker(nil, nil)
		chk.Overrides = map[string]bool{"1==2": false}
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx, req := statusReq(exec, s1)
		if err := v.Handle(ctx, req); err != nil {
			t.Fatalf("s1 should be allowed once s2 is proven skipped: %v", err)
		}
	})
}

// TestOrderingVerifier_UnresolvableGuardFailsClosed pins the uniform fail-closed
// behavior over an *unresolvable* guard (nil evaluator, no matching override, so
// s2's condition can't be decided). There is no fail-open opt-out: s2's own
// unresolvable guard denies s2, and the ancestor walk-up never steps over the
// unknown s2, so s3 stays blocked on it. A known-false override, by contrast, is
// a proven skip and steps over.
func TestOrderingVerifier_UnresolvableGuardFailsClosed(t *testing.T) {
	const exec = "6LzXi5kqRS-unresolvable"
	s1 := "pipeline.stages.st1.spec.execution.steps.s1"
	s2 := "pipeline.stages.st1.spec.execution.steps.s2"
	s3 := "pipeline.stages.st1.spec.execution.steps.s3"

	t.Run("unresolvable s2 is denied and never stepped over", func(t *testing.T) {
		chk := NewConditionChecker(nil, nil) // no evaluator, no overrides -> s2 unresolvable
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx1, req1 := skipReq(exec, s1)
		if err := v.Handle(ctx1, req1); err != nil {
			t.Fatalf("s1 (entry) should be allowed: %v", err)
		}
		// s2's own guard is unresolvable -> denied (fail closed).
		ctx2, req2 := skipReq(exec, s2)
		if err := v.Handle(ctx2, req2); err == nil {
			t.Fatal("s2 must be denied on an unresolvable guard")
		}
		// s2 never ran; the walk-up doesn't step over an unknown ancestor -> s3
		// stays blocked on the still-required s2.
		ctx3, req3 := skipReq(exec, s3)
		if err := v.Handle(ctx3, req3); err == nil {
			t.Fatal("s3 must stay blocked on the still-required s2")
		}
	})

	t.Run("known-false override is a proven skip and steps over", func(t *testing.T) {
		chk := NewConditionChecker(nil, nil)
		chk.Overrides = map[string]bool{`<+pipeline.variables.run> == "yes"`: false}
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx1, req1 := skipReq(exec, s1)
		if err := v.Handle(ctx1, req1); err != nil {
			t.Fatalf("s1 (entry) should be allowed: %v", err)
		}
		ctx3, req3 := skipReq(exec, s3)
		if err := v.Handle(ctx3, req3); err != nil {
			t.Fatalf("known-false override: s3 should be allowed over the proven-skipped s2: %v", err)
		}
	})
}

// TestOrderingVerifier_SkippedAncestorWalkUp: s3 is allowed after s1 even though
// the intervening s2 is skipped by a false `when` — a follower is not blocked on
// a step that never runs.
func TestOrderingVerifier_SkippedAncestorWalkUp(t *testing.T) {
	const exec = "6LzXi5kqRS-skipwalkup"
	s1 := "pipeline.stages.st1.spec.execution.steps.s1"
	s3 := "pipeline.stages.st1.spec.execution.steps.s3"

	// run != "yes" -> s2 skipped.
	chk := NewConditionChecker(demoRunEval, map[string]string{"pipeline.variables.run": "no"})
	v, err := New(Config{Conditions: chk})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx1, req1 := skipReq(exec, s1)
	if err := v.Handle(ctx1, req1); err != nil {
		t.Fatalf("s1 (entry) should be allowed: %v", err)
	}
	// s3 should still be allowed on s1 alone (s2 never verified).
	ctx3, req3 := skipReq(exec, s3)
	if err := v.Handle(ctx3, req3); err != nil {
		t.Fatalf("s3 should be allowed once s1 ran and s2 is skipped: %v", err)
	}
}

// TestOrderingVerifier_ConfigOverrideWalkUp: a config-supplied ConditionOverrides
// map (no code-wired checker) forces s2's guard false, so s3 is allowed after s1
// with the skipped s2 stepped over.
func TestOrderingVerifier_ConfigOverrideWalkUp(t *testing.T) {
	const exec = "6LzXi5kqRS-cfgoverride"
	s1 := "pipeline.stages.st1.spec.execution.steps.s1"
	s3 := "pipeline.stages.st1.spec.execution.steps.s3"

	v, err := New(Config{ConditionOverrides: map[string]bool{
		`<+pipeline.variables.run> == "yes"`: false, // force-skip s2, no evaluator
	}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx1, req1 := skipReq(exec, s1)
	if err := v.Handle(ctx1, req1); err != nil {
		t.Fatalf("s1 (entry) should be allowed: %v", err)
	}
	ctx3, req3 := skipReq(exec, s3)
	if err := v.Handle(ctx3, req3); err != nil {
		t.Fatalf("s3 should be allowed once s1 ran and s2 is overridden-skipped: %v", err)
	}
}

// TestOrderingVerifier_ConditionEnforced: the `when` guard on s2 is enforced on
// top of ordering — s2 is allowed only when its condition is true and s1 has run.
func TestOrderingVerifier_ConditionEnforced(t *testing.T) {
	const exec = "6LzXi5kqRS-pDtlM1cWSBw"
	s1 := "pipeline.stages.st1.spec.execution.steps.s1"
	s2 := "pipeline.stages.st1.spec.execution.steps.s2"

	t.Run("condition true -> s2 allowed after s1", func(t *testing.T) {
		chk := NewConditionChecker(demoRunEval, map[string]string{"pipeline.variables.run": "yes"})
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx1, req1 := stepReq(exec, s1)
		if err := v.Handle(ctx1, req1); err != nil {
			t.Fatalf("s1 (entry) should be allowed: %v", err)
		}
		ctx2, req2 := stepReq(exec, s2)
		if err := v.Handle(ctx2, req2); err != nil {
			t.Fatalf("s2 should be allowed once s1 ran and condition holds: %v", err)
		}
	})

	t.Run("condition false -> s2 denied even after s1", func(t *testing.T) {
		chk := NewConditionChecker(demoRunEval, map[string]string{"pipeline.variables.run": "no"})
		v, err := New(Config{Conditions: chk})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx1, req1 := stepReq(exec, s1)
		if err := v.Handle(ctx1, req1); err != nil {
			t.Fatalf("s1 should be allowed: %v", err)
		}
		ctx2, req2 := stepReq(exec, s2)
		if err := v.Handle(ctx2, req2); err == nil {
			t.Fatal("s2 must be denied when its when-condition is false")
		}
	})

	t.Run("no checker -> condition unenforced (ordering only)", func(t *testing.T) {
		v, err := New(Config{}) // Conditions nil => AlwaysTrue
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ctx1, req1 := stepReq(exec, s1)
		if err := v.Handle(ctx1, req1); err != nil {
			t.Fatalf("s1 should be allowed: %v", err)
		}
		ctx2, req2 := stepReq(exec, s2)
		if err := v.Handle(ctx2, req2); err != nil {
			t.Fatalf("without a checker, s2 should pass on ordering alone: %v", err)
		}
	})
}
