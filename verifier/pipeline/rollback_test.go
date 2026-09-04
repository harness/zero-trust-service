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

// TestRollbackSubDAG verifies stage rollbackSteps as a failure-path sub-DAG (case34): they resolve, are ordered among themselves, and OR-gate their entry on forward steps while no forward step gates on a rollback step.
func TestRollbackSubDAG(t *testing.T) {
	yaml := readFixture(t, "case34_stage_rollback.yaml")
	g, err := BuildGraph(yaml)
	if err != nil || g == nil {
		t.Fatalf("BuildGraph returned (%v, %v)", g, err)
	}
	fwd := func(id string) string { return "pipeline.stages.st1.spec.execution.steps." + id }
	rb := func(id string) string { return "pipeline.stages.st1.spec.execution.rollbackSteps." + id }

	t.Run("rollback steps resolve instead of reporting not-found", func(t *testing.T) {
		for _, id := range []string{"rb_deploy", "rb_cleanup"} {
			if res := g.Resolve(rb(id)); !res.Found {
				t.Errorf("Resolve(%s) not found; rollback steps must be indexed", rb(id))
			}
		}
	})

	t.Run("rollback entry OR-gates on the stage's forward steps", func(t *testing.T) {
		res := g.Resolve(rb("rb_deploy"))
		if res.Join != JoinOR {
			t.Errorf("rb_deploy join = %v, want OR (failure-triggered)", res.Join)
		}
		var got []string
		for _, a := range res.Ancestors {
			got = append(got, a.FQN)
		}
		if !sameSet(got, []string{fwd("s1"), fwd("s2")}) {
			t.Fatalf("rb_deploy should gate on forward leaves [s1, s2], got %+v", res.Ancestors)
		}
		// Denied when no forward step has run: nothing to roll back.
		if v, _ := g.EvaluateStep(rb("rb_deploy"), WithRan(runSet())); v.Allowed {
			t.Fatalf("rb_deploy must be denied when no forward step ran: %+v", v)
		}
		// Allowed once ANY forward step has run (OR).
		if v, _ := g.EvaluateStep(rb("rb_deploy"), WithRan(runSet(fwd("s1")))); !v.Allowed {
			t.Fatalf("rb_deploy should be allowed once a forward step ran: %+v", v)
		}
	})

	t.Run("rollback steps are ordered among themselves", func(t *testing.T) {
		res := g.Resolve(rb("rb_cleanup"))
		if len(res.Ancestors) != 1 || res.Ancestors[0].FQN != rb("rb_deploy") {
			t.Fatalf("rb_cleanup should wait on rb_deploy, got %+v", res.Ancestors)
		}
		if v, _ := g.EvaluateStep(rb("rb_cleanup"), WithRan(runSet())); v.Allowed {
			t.Fatalf("rb_cleanup must be denied until rb_deploy has run: %+v", v)
		}
		if v, _ := g.EvaluateStep(rb("rb_cleanup"), WithRan(runSet(rb("rb_deploy")))); !v.Allowed {
			t.Fatalf("rb_cleanup should be allowed once rb_deploy ran: %+v", v)
		}
	})

	t.Run("forward flow is unaffected by the rollback sub-DAG", func(t *testing.T) {
		res := g.Resolve(fwd("s2"))
		if len(res.Ancestors) != 1 || res.Ancestors[0].FQN != fwd("s1") {
			t.Fatalf("s2 should wait only on s1, got %+v", res.Ancestors)
		}
		// A forward entry step must not pick up any rollback dependency.
		if res := g.Resolve(fwd("s1")); len(res.Ancestors) != 0 {
			t.Fatalf("s1 should be a forward entry with no ancestors, got %+v", res.Ancestors)
		}
	})

	// A StageRollback rollback shares the forward run's execution id, so its run-state is visible: no separate-plan flag.
	t.Run("stage-rollback steps are not flagged as separate-plan", func(t *testing.T) {
		if res := g.Resolve(rb("rb_deploy")); res.SeparatePlanRollback {
			t.Fatal("StageRollback rollback must not be flagged SeparatePlanRollback")
		}
	})
}

// TestPipelineRollbackOutOfScope: with PipelineRollback (case35) the rollback runs in a separate plan, so only the rollback entry is flagged SeparatePlanRollback (callers fail open); later rollback steps and the forward flow are ordered normally.
func TestPipelineRollbackOutOfScope(t *testing.T) {
	g, err := BuildGraph(readFixture(t, "case35_pipeline_rollback.yaml"))
	if err != nil || g == nil {
		t.Fatalf("BuildGraph returned (%v, %v)", g, err)
	}
	fwd := func(id string) string { return "pipeline.stages.st1.spec.execution.steps." + id }
	rb := func(id string) string { return "pipeline.stages.st1.spec.execution.rollbackSteps." + id }

	t.Run("only the rollback entry is flagged separate-plan", func(t *testing.T) {
		if res := g.Resolve(rb("rb_deploy")); !res.Found || !res.SeparatePlanRollback {
			t.Errorf("entry rb_deploy should be flagged SeparatePlanRollback, got %+v", res)
		}
		// A later rollback step gates within the same plan, so it is ordered normally and NOT flagged.
		if res := g.Resolve(rb("rb_cleanup")); !res.Found || res.SeparatePlanRollback {
			t.Errorf("non-entry rb_cleanup must not be flagged SeparatePlanRollback, got %+v", res)
		}
	})

	t.Run("entry gate is short-circuited (fail open)", func(t *testing.T) {
		// No forward step ran, yet the entry must not deny: it surfaces the flag and leaves gating to the caller (fail open).
		v, _ := g.EvaluateStep(rb("rb_deploy"), WithRan(runSet()))
		if !hasPipelineRollback(v) {
			t.Errorf("entry verdict should carry SeparatePlanRollback, got %+v", v)
		}
		if len(missingAncestors(v)) != 0 {
			t.Errorf("flagged entry must not report missing ancestors, got %v", missingAncestors(v))
		}
	})

	t.Run("later rollback steps are still ordered within the plan", func(t *testing.T) {
		// rb_cleanup gates on rb_deploy having run in the same (rollback) plan.
		if v, _ := g.EvaluateStep(rb("rb_cleanup"), WithRan(runSet())); v.Allowed {
			t.Fatalf("rb_cleanup must be denied until rb_deploy ran in the rollback plan: %+v", v)
		}
		if v, _ := g.EvaluateStep(rb("rb_cleanup"), WithRan(runSet(rb("rb_deploy")))); !v.Allowed {
			t.Fatalf("rb_cleanup should be allowed once rb_deploy ran: %+v", v)
		}
	})

	t.Run("forward flow is still ordered normally", func(t *testing.T) {
		if res := g.Resolve(fwd("s2")); res.SeparatePlanRollback {
			t.Fatal("forward step must not be flagged SeparatePlanRollback")
		}
		if v, _ := g.EvaluateStep(fwd("s2"), WithRan(runSet())); v.Allowed {
			t.Fatalf("s2 must be denied until s1 runs, got %+v", v)
		}
		if v, _ := g.EvaluateStep(fwd("s2"), WithRan(runSet(fwd("s1")))); !v.Allowed {
			t.Fatalf("s2 should be allowed once s1 ran, got %+v", v)
		}
	})
}

// rollbackEntryOwnGuardYAML: a PipelineRollback stage whose rollback ENTRY (rb_deploy)
// carries its own `when` guard. The forward gate is unknowable (separate plan), but
// the entry's OWN guard is locally knowable — a definitively-false guard must still
// block, even when the rollback policy accepts the forward gate. Regression for the
// short-circuit that discarded the entry's own facts.
const rollbackEntryOwnGuardYAML = `
pipeline:
  identifier: prbguard
  stages:
    - stage:
        identifier: st1
        type: Deployment
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: ShellScript
            rollbackSteps:
              - step:
                  identifier: rb_deploy
                  type: ShellScript
                  when:
                    stageStatus: Failure
                    condition: "<+pipeline.variables.doRollback> == \"true\""
              - step:
                  identifier: rb_cleanup
                  type: ShellScript
        failureStrategies:
          - onFailure:
              errors:
                - AllErrors
              action:
                type: PipelineRollback
`

func TestRollbackEntryOwnGuardStillEvaluated(t *testing.T) {
	const rb = "pipeline.stages.st1.spec.execution.rollbackSteps.rb_deploy"
	g, err := BuildGraph(rollbackEntryOwnGuardYAML)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	res := g.Resolve(rb)
	if !res.Found || !res.SeparatePlanRollback {
		t.Fatalf("rb_deploy should resolve as a SeparatePlanRollback entry: %+v", res)
	}

	// Accept the forward gate (the reason the entry can't be ordered), but drive the
	// entry's OWN condition definitively false.
	acceptForward := WithPipelineRollbackPolicy(func(*PipelineRollbackViolation) bool { return true })
	guardFalse := func(w When) (bool, error) { return w.Expression == "", nil } // expression guard fails

	v, _ := g.EvaluateStep(rb, WithRan(ranSet()), WithCondition(guardFalse), acceptForward)
	if v.Allowed {
		t.Fatalf("rb_deploy must be denied: its own guard did not hold, forward gate aside: %+v", v)
	}
	if firstCondition(v) == nil {
		t.Fatalf("expected a ConditionViolation for the entry's own guard, got %+v", v.Violations)
	}
	if !hasPipelineRollback(v) {
		t.Errorf("the forward gate should still surface as a PipelineRollbackViolation: %+v", v.Violations)
	}

	// With the guard holding and the forward gate accepted, the entry is allowed.
	guardHolds := func(When) (bool, error) { return true, nil }
	if vv, _ := g.EvaluateStep(rb, WithRan(ranSet()), WithCondition(guardHolds), acceptForward); !vv.Allowed {
		t.Fatalf("rb_deploy should be allowed when its guard holds and the forward gate is accepted: %+v", vv)
	}
}
