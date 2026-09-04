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

import (
	"errors"
	"testing"
)

// skipChainYAML is a chain s1 -> s2 -> s3 where s2 carries a `when`; when s2 is
// skipped, s3's AND gate falls through to s1. Walk-up over skipped ancestors is
// unconditional for AND joins, so a follower is never stuck on a unit that never runs.
const skipChainYAML = `
pipeline:
  identifier: pskip
  stages:
    - stage:
        identifier: st
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: Run
              - step:
                  identifier: s2
                  type: Run
                  when:
                    stageStatus: Success
                    condition: "<+run> == \"yes\""
              - step:
                  identifier: s3
                  type: Run
`

func TestSkippedAncestorWalkUp(t *testing.T) {
	const base = "pipeline.stages.st.spec.execution.steps."
	s1, s2, s3 := base+"s1", base+"s2", base+"s3"

	// cond that skips s2 (its when is known not to hold) but leaves every other guard alone.
	skipsS2 := func(w When) (bool, error) { return w.Expression == "", nil }

	t.Run("s3 gates on s1 when s2 is skipped", func(t *testing.T) {
		g, _ := BuildGraph(skipChainYAML)
		// s2 is skipped -> s3's effective ancestor is s1, so s1 having run is enough.
		if v, _ := g.EvaluateStep(s3, WithRan(ranSet(s1)), WithCondition(skipsS2)); !v.Allowed {
			t.Fatalf("expected s3 allowed once s1 ran and s2 is skipped: %+v", v)
		}
		// s1 has not run yet -> s3 still blocked, now reporting s1 (not the skipped s2).
		v, _ := g.EvaluateStep(s3, WithRan(ranSet()), WithCondition(skipsS2))
		if v.Allowed {
			t.Fatalf("expected s3 blocked while s1 un-run: %+v", v)
		}
		if !sortedEq(missingAncestors(v), []string{s1}) {
			t.Errorf("MissingAncestors = %v, want [%s]", missingAncestors(v), s1)
		}
	})

	t.Run("s2 live (no skip): s3 still gates on s2", func(t *testing.T) {
		g, _ := BuildGraph(skipChainYAML)
		// Default cond (AlwaysTrue) skips nothing, so normal ordering: s3 needs s2, and s1 alone is not enough.
		if v, _ := g.EvaluateStep(s3, WithRan(ranSet(s1))); v.Allowed {
			t.Fatalf("expected s3 blocked on live-but-un-run s2: %+v", v)
		}
		if v, _ := g.EvaluateStep(s3, WithRan(ranSet(s1, s2))); !v.Allowed {
			t.Fatalf("expected s3 allowed once s2 ran: %+v", v)
		}
	})
}

// skipTwoYAML chains s1 -> s2 -> s3 -> s4 with guards on s2 and s3 to exercise a transitive walk-up (two consecutive skips) landing s4 on s1.
const skipTwoYAML = `
pipeline:
  identifier: pskip2
  stages:
    - stage:
        identifier: st
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: Run
              - step:
                  identifier: s2
                  type: Run
                  when:
                    condition: "<+a> == \"yes\""
              - step:
                  identifier: s3
                  type: Run
                  when:
                    condition: "<+b> == \"yes\""
              - step:
                  identifier: s4
                  type: Run
`

func TestSkippedAncestorWalkUp_Transitive(t *testing.T) {
	const base = "pipeline.stages.st.spec.execution.steps."
	s1, s4 := base+"s1", base+"s4"
	g, _ := BuildGraph(skipTwoYAML)

	// Both s2 and s3 skipped -> s4 must fall all the way through to s1.
	skipGuarded := func(w When) (bool, error) { return w.Expression == "", nil }
	if v, _ := g.EvaluateStep(s4, WithRan(ranSet(s1)), WithCondition(skipGuarded)); !v.Allowed {
		t.Fatalf("expected s4 allowed via transitive walk-up to s1: %+v", v)
	}
	v, _ := g.EvaluateStep(s4, WithRan(ranSet()), WithCondition(skipGuarded))
	if v.Allowed || !sortedEq(missingAncestors(v), []string{s1}) {
		t.Fatalf("expected s4 blocked reporting only s1: %+v", v)
	}
}

// orExcludedAncestorYAML: s1 -> s2 (matrix excludes every combination) -> onfail
// (`when stageStatus: Failure`, an OR join). onfail's immediate predecessor s2 never
// runs, so it must be stepped over and onfail gated on s2's live predecessor s1 —
// not vacuously satisfied. Regression for the OR-path fail-open: before the fix an
// OR follower whose only candidate was an all-excluded matrix was silently allowed.
const orExcludedAncestorYAML = `
pipeline:
  identifier: porexcl
  stages:
    - stage:
        identifier: st
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
                  strategy:
                    matrix:
                      a: [x, y]
                      b: [p, q]
                      exclude:
                        - {a: x, b: p}
                        - {a: x, b: q}
                        - {a: y, b: p}
                        - {a: y, b: q}
              - step:
                  identifier: onfail
                  type: ShellScript
                  when:
                    stageStatus: Failure
`

func TestORStepsOverExcludedAncestor(t *testing.T) {
	const base = "pipeline.stages.st.spec.execution.steps."
	s1, onfail := base+"s1", base+"onfail"
	g, err := BuildGraph(orExcludedAncestorYAML)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Nothing run: onfail's excluded predecessor is stepped over to s1, which has not
	// run, so the OR is unsatisfied and onfail is denied — reporting s1, not empty.
	v, _ := g.EvaluateStep(onfail, WithRan(ranSet()))
	if v.Allowed {
		t.Fatalf("onfail must be denied while s1 (its live predecessor) has not run: %+v", v)
	}
	if !sortedEq(missingAncestors(v), []string{s1}) {
		t.Errorf("MissingAncestors = %v, want [%s] (stepped over the excluded s2)", missingAncestors(v), s1)
	}

	// s1 ran: the OR fires on its one live candidate.
	if v, _ := g.EvaluateStep(onfail, WithRan(ranSet(s1))); !v.Allowed {
		t.Fatalf("onfail should be allowed once s1 ran (OR satisfied): %+v", v)
	}
}

// ancestorUnknownGuardYAML: s1 carries a `when` expression; s2 gates on s1 (AND).
// When the ConditionFunc cannot evaluate s1's guard, the SDK can't tell whether s1
// was required — that must surface as a ConditionUnknownViolation naming s1 (relaxable
// via WithConditionUnknownPolicy), not as a blanket AncestorDidNotRunViolation whose
// only escape is the far broader WithAncestorPolicy.
const ancestorUnknownGuardYAML = `
pipeline:
  identifier: pancunk
  stages:
    - stage:
        identifier: st
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: ShellScript
                  when:
                    condition: "<+pipeline.variables.x> == \"go\""
              - step:
                  identifier: s2
                  type: ShellScript
`

func TestAncestorGuardUnknownIsConditionUnknown(t *testing.T) {
	const base = "pipeline.stages.st.spec.execution.steps."
	s1, s2 := base+"s1", base+"s2"
	g, err := BuildGraph(ancestorUnknownGuardYAML)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// A ConditionFunc that errors on s1's expression guard (a resolver gap).
	unresolvable := func(w When) (bool, error) {
		if w.Expression != "" {
			return false, errors.New("expression resolver unavailable")
		}
		return true, nil
	}

	// Fail-closed by default, and the blocking violation is a ConditionUnknownViolation
	// naming s1 — not an AncestorDidNotRunViolation.
	v, _ := g.EvaluateStep(s2, WithRan(ranSet()), WithCondition(unresolvable))
	if v.Allowed {
		t.Fatalf("s2 must be denied while s1's guard cannot be evaluated: %+v", v)
	}
	var cu *ConditionUnknownViolation
	for _, vi := range v.Violations {
		if c := AsConditionUnknownViolation(vi); c != nil {
			cu = c
		}
		if m := AsAncestorDidNotRunViolation(vi); m != nil {
			t.Errorf("s1 should not be a plain AncestorDidNotRunViolation: %+v", m)
		}
	}
	if cu == nil {
		t.Fatalf("expected a ConditionUnknownViolation for s1's guard, got %+v", v.Violations)
	}
	if cu.FQN != s1 {
		t.Errorf("ConditionUnknownViolation.FQN = %q, want %q", cu.FQN, s1)
	}

	// The unknown-specific policy relaxes it; the blunt ancestor policy is not needed.
	acceptUnknown := WithConditionUnknownPolicy(func(*ConditionUnknownViolation) bool { return true })
	if vv, _ := g.EvaluateStep(s2, WithRan(ranSet()), WithCondition(unresolvable), acceptUnknown); !vv.Allowed {
		t.Fatalf("WithConditionUnknownPolicy should allow s2: %+v", vv)
	}
}
