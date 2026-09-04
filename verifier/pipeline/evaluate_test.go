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
	"fmt"
	"sort"
	"testing"
)

func ranSet(fqns ...string) StepRanFunc {
	set := make(map[string]bool, len(fqns))
	for _, f := range fqns {
		set[f] = true
	}
	return func(fqn string) (bool, error) { return set[fqn], nil }
}

func sortedEq(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return eq(a, b)
}

// The SDK deliberately exposes no projection helpers over Verdict.Violations — a
// consumer iterates and switches on the concrete kind. These test-only helpers
// reproduce the old projections so the assertions stay terse; they double as a
// worked example of the intended iterate-and-switch pattern.
func blocking(v Verdict) []Violation {
	var out []Violation
	for _, vi := range v.Violations {
		if !vi.IsIgnored() {
			out = append(out, vi)
		}
	}
	return out
}

// missingAncestors aggregates the FQNs across every AncestorDidNotRunViolation on
// the verdict. Under an AND join the SDK emits one violation per unsatisfied
// ancestor (each carrying its own still-missing instance FQNs), so a caller that
// wants the whole missing set collects across all of them.
func missingAncestors(v Verdict) []string {
	var out []string
	for _, vi := range v.Violations {
		if m := AsAncestorDidNotRunViolation(vi); m != nil {
			out = append(out, m.Ancestors...)
		}
	}
	return out
}

// runtimeFanouts collects the FQNs of every RuntimeFanoutViolation on the verdict
// (a node — ancestor or the evaluated step — whose runtime fan-out is not statically
// enumerable).
func runtimeFanouts(v Verdict) []string {
	var out []string
	for _, vi := range v.Violations {
		if r := AsRuntimeFanoutViolation(vi); r != nil {
			out = append(out, r.FQN)
		}
	}
	return out
}

func hasPipelineRollback(v Verdict) bool {
	for _, vi := range v.Violations {
		if IsPipelineRollbackViolation(vi) {
			return true
		}
	}
	return false
}

func conditionUnmet(v Verdict) bool {
	for _, vi := range v.Violations {
		if IsConditionViolation(vi) {
			return true
		}
	}
	return false
}

// conditionViolations returns every ConditionViolation on the verdict (one is
// emitted per unmet guard).
func conditionViolations(v Verdict) []*ConditionViolation {
	var out []*ConditionViolation
	for _, vi := range v.Violations {
		if c := AsConditionViolation(vi); c != nil {
			out = append(out, c)
		}
	}
	return out
}

// ancestorFQNs projects the FQNs of a verdict's effective ancestor set.
func ancestorFQNs(v Verdict) []string {
	out := make([]string, len(v.Ancestors))
	for i, a := range v.Ancestors {
		out[i] = a.FQN
	}
	return out
}

// orYAML: a step whose own `when` targets Failure, so it resolves to an OR join over its predecessor.
const orYAML = `
pipeline:
  identifier: por
  stages:
    - stage:
        identifier: s
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: main
                  type: Run
              - step:
                  identifier: onfail
                  type: Run
                  when:
                    stageStatus: Failure
`

func TestEvaluateStep_AndAndEntry(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const base = "pipeline.stages.build.spec.execution.steps."

	tests := []struct {
		name    string
		fqn     string
		ran     StepRanFunc
		found   bool
		allowed bool
		missing []string
	}{
		{"entry step allowed", base + "s1", nil, true, true, nil},
		{"AND all ancestors ran", base + "s2", ranSet(base+"p_a", base+"p_b"), true, true, nil},
		{"AND missing one denied", base + "s2", ranSet(base + "p_a"), true, false, []string{base + "p_b"}},
		{"AND nil ran denies all", base + "s2", nil, true, false, []string{base + "p_a", base + "p_b"}},
		{"unknown fqn not found", base + "nope", nil, false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// no WithCondition => AlwaysTrue, so only ordering is under test.
			v, err := g.EvaluateStep(tt.fqn, WithRan(tt.ran))
			// A missing step is signaled solely by ErrNodeNotFound (there is no
			// Verdict.Found); a found step returns a nil error.
			if found := !errors.Is(err, ErrNodeNotFound); found != tt.found {
				t.Fatalf("found = %v, want %v (%+v, err %v)", found, tt.found, v, err)
			}
			if v.Allowed != tt.allowed {
				t.Errorf("Allowed = %v, want %v (violations %v)", v.Allowed, tt.allowed, v.Violations)
			}
			if tt.found && v.Join != JoinAND {
				t.Errorf("Join = %v, want JoinAND", v.Join)
			}
			if !sortedEq(missingAncestors(v), tt.missing) {
				t.Errorf("MissingAncestors = %v, want %v", missingAncestors(v), tt.missing)
			}
		})
	}
}

func TestEvaluateStep_NotFound(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	v, err := g.EvaluateStep("pipeline.stages.build.spec.execution.steps.nope")
	if v.Allowed || !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("unexpected verdict for missing step: %+v (err %v)", v, err)
	}
}

func TestEvaluateStep_ConditionFalseDenies(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const base = "pipeline.stages.build.spec.execution.steps."
	// s2 has its own `when`; a never-holding cond must block it even with both ancestors run.
	falseCond := func(When) (bool, error) { return false, nil } // known false
	v, _ := g.EvaluateStep(base+"s2", WithRan(ranSet(base+"p_a", base+"p_b")), WithCondition(falseCond))
	if v.Allowed {
		t.Errorf("expected denial on unsatisfied condition, got %+v", v)
	}
	if !conditionUnmet(v) {
		t.Errorf("expected a ConditionViolation, got %+v", v.Violations)
	}
}

// An unmet guard and an unevaluable one are distinct: a guard that evaluated and
// did not hold is a ConditionViolation (carrying its structured guard: GuardFQN,
// Kind, Expression/Status); a guard the ConditionFunc could not evaluate (a non-nil
// error) is a ConditionUnknownViolation instead, carrying the same structure plus
// Err. Both fail closed, but a caller can tell them apart and relax them separately.
func TestEvaluateStep_ConditionViolationCarriesStructuredGuard(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const base = "pipeline.stages.build.spec.execution.steps."
	ran := ranSet(base+"p_a", base+"p_b") // ancestors satisfied, so the only violation is the condition

	// Unevaluable guard (cond returns an error): a ConditionUnknownViolation, whose
	// Err surfaces the failure and whose guard is carried structurally.
	boom := errors.New("resolver unavailable")
	unknown := func(When) (bool, error) { return false, boom }
	v, _ := g.EvaluateStep(base+"s2", WithRan(ran), WithCondition(unknown))
	var cu *ConditionUnknownViolation
	for _, vi := range v.Violations {
		if c := AsConditionUnknownViolation(vi); c != nil {
			cu = c
		}
	}
	if cu == nil {
		t.Fatalf("expected a ConditionUnknownViolation, got %+v", v.Violations)
	}
	if !errors.Is(cu.Err(), boom) {
		t.Errorf("ConditionUnknownViolation.Err() = %v, want %v", cu.Err(), boom)
	}
	if cu.GuardFQN == "" || cu.FQN == "" {
		t.Errorf("violation should carry the guard's owner and the evaluated node, got %+v", cu)
	}
	if len(conditionViolations(v)) != 0 {
		t.Errorf("an unevaluable guard should not also raise a ConditionViolation: %+v", v.Violations)
	}

	// Known-false guard (cond returns false, nil): a ConditionViolation, carrying the
	// guard structurally so a caller can inspect it without string-matching.
	knownFalse := func(When) (bool, error) { return false, nil }
	v2, _ := g.EvaluateStep(base+"s2", WithRan(ran), WithCondition(knownFalse))
	cvs2 := conditionViolations(v2)
	if len(cvs2) == 0 {
		t.Fatalf("expected a ConditionViolation, got %+v", v2.Violations)
	}
	for _, cv := range cvs2 {
		if cv.GuardFQN == "" {
			t.Errorf("violation should carry the guard owner FQN, got %+v", cv)
		}
		if cv.Kind != ConditionExpression && cv.Kind != ConditionStatus {
			t.Errorf("guard Kind = %q, want expression or status", cv.Kind)
		}
		if cv.Kind == ConditionExpression && cv.Expression == "" {
			t.Errorf("an expression guard should carry its Expression, got %+v", cv)
		}
	}
}

// strategyYAML builds a two-step stage whose first step carries a strategy; the
// %s is spliced in as that step's strategy body so a test can pick runtime-input
// vs malformed fan-out. Step two depends on step one (AND join over it).
const strategyYAML = `
pipeline:
  identifier: pstrat
  stages:
    - stage:
        identifier: s
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: first
                  type: Run
                  strategy:
%s
              - step:
                  identifier: second
                  type: Run
`

// TestEvaluateStep_RuntimeFanoutAncestor: an ancestor whose fan-out is a benign
// runtime <+input> count is not statically enumerable, so its fan-in can never be
// verified complete — a fail-closed RuntimeFanoutViolation on that node whenever it
// gates the step, relaxable per occurrence with WithRuntimeFanoutPolicy.
func TestEvaluateStep_RuntimeFanoutAncestor(t *testing.T) {
	const base = "pipeline.stages.s.spec.execution.steps."
	second := base + "second"
	first := base + "first"

	g, err := BuildGraph(fmt.Sprintf(strategyYAML, "                    matrix: <+input>"))
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	v, _ := g.EvaluateStep(second)
	if v.Allowed {
		t.Fatalf("runtime-input ancestor should fail closed by default: %+v", v)
	}
	if rf := runtimeFanouts(v); len(rf) != 1 || rf[0] != first {
		t.Errorf("RuntimeFanouts = %v, want [%s]", rf, first)
	}
	r := AsRuntimeFanoutViolation(blocking(v)[0])
	if r == nil || r.FQN != first || r.Status != FanoutRuntimeInput {
		t.Errorf("want a RuntimeFanoutViolation on %s with Status=%s, got %+v", first, FanoutRuntimeInput, r)
	}
	// A policy accepting runtime fan-out lets it through; the policy sees the node's
	// FQN and Status so it could decide per occurrence.
	failOpen := WithRuntimeFanoutPolicy(func(rv *RuntimeFanoutViolation) bool {
		return rv.FQN == first && rv.Status == FanoutRuntimeInput
	})
	if vo, _ := g.EvaluateStep(second, failOpen); !vo.Allowed {
		t.Errorf("runtime-fanout policy should allow it: %+v", vo)
	}
}

// TestBuildGraph_MalformedStrategyErrors: a structurally malformed strategy (here a
// matrix axis whose value is a scalar, not a sequence) is no longer a runtime
// violation — it is rejected at build time, so BuildGraph fails closed with
// ErrInvalidPipeline and no graph is produced to evaluate against.
func TestBuildGraph_MalformedStrategyErrors(t *testing.T) {
	_, err := BuildGraph(fmt.Sprintf(strategyYAML, "                    matrix:\n                      axis: notasequence"))
	if !errors.Is(err, ErrInvalidPipeline) {
		t.Errorf("BuildGraph on a malformed matrix axis: err = %v, want ErrInvalidPipeline", err)
	}
}

// skipCond makes a `when` whose Expression is "1==2" provably not hold (skip),
// leaving every other guard holding. Used to force a step to be stepped over.
func skipCond(w When) (bool, error) {
	if w.Expression == "1==2" {
		return false, nil
	}
	return true, nil
}

// TestLiveAncestors_SkipWalkPrunesRedundant: stepping over a skipped predecessor
// climbs its branch deeper than one level. The resulting live frontier must stay
// minimal — a plain sequential chain gates only on its direct predecessor (the
// rest is implied inductively), and the skip-walk must not surface an upstream
// node a live sibling already implies. It must still keep an upstream node that
// no sibling covers.
func TestLiveAncestors_SkipWalkPrunesRedundant(t *testing.T) {
	const b = "pipeline.stages.st1.spec.execution.steps."

	// tc22 shape: ShellScript_4 -> parallel{ s2(skipped), ShellScript_3 } -> s1.
	// s2 skips to its pre-parallel predecessor ShellScript_4, but the live sibling
	// ShellScript_3 (ShellScript_4 -> ShellScript_3, AND) already implies it, so
	// ShellScript_4 is pruned: the frontier is just ShellScript_3.
	const shared = `
pipeline:
  identifier: tc22
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step: {identifier: ShellScript_4, type: ShellScript}
              - parallel:
                  - step: {identifier: s2, type: ShellScript, when: {stageStatus: Success, condition: "1==2"}}
                  - step: {identifier: ShellScript_3, type: ShellScript}
              - step: {identifier: s1, type: ShellScript}
`
	// Sequential sole-path: ShellScript_4 -> s2(skipped) -> s1. No sibling to carry
	// the ShellScript_4 gate, so the climb is retained.
	const sole = `
pipeline:
  identifier: seq
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step: {identifier: ShellScript_4, type: ShellScript}
              - step: {identifier: s2, type: ShellScript, when: {stageStatus: Success, condition: "1==2"}}
              - step: {identifier: s1, type: ShellScript}
`
	// Skipped parallel branch with its OWN live upstream X (X -> s2 inside one
	// branch, ShellScript_3 the other). The engine still runs X, so s1 must wait on
	// it; ShellScript_3 does not cover X, so X is kept.
	const ownUpstream = `
pipeline:
  identifier: par
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step: {identifier: ShellScript_4, type: ShellScript}
              - parallel:
                  - stepGroup:
                      identifier: g
                      steps:
                        - step: {identifier: X, type: ShellScript}
                        - step: {identifier: s2, type: ShellScript, when: {stageStatus: Success, condition: "1==2"}}
                  - step: {identifier: ShellScript_3, type: ShellScript}
              - step: {identifier: s1, type: ShellScript}
`
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{"shared upstream pruned", shared, []string{b + "ShellScript_3"}},
		{"sole-path climb kept", sole, []string{b + "ShellScript_4"}},
		{"own-upstream kept", ownUpstream, []string{b + "g.steps.X", b + "ShellScript_3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := BuildGraph(tt.yaml)
			if err != nil {
				t.Fatalf("BuildGraph: %v", err)
			}
			v, _ := g.EvaluateStep(b+"s1", WithCondition(skipCond))
			if !sortedEq(ancestorFQNs(v), tt.want) {
				t.Errorf("Ancestors = %v, want %v", ancestorFQNs(v), tt.want)
			}
		})
	}

	// A retained upstream still gates: in the own-upstream shape, ShellScript_3
	// running but X pending must NOT allow s1 (pruning never weakens the gate).
	g, _ := BuildGraph(ownUpstream)
	if v, _ := g.EvaluateStep(b+"s1", WithCondition(skipCond), WithRan(ranSet(b+"ShellScript_3"))); v.Allowed {
		t.Errorf("s1 should be denied while X is pending: %+v", v)
	}
}

func TestEvaluateStep_OrFailureBranch(t *testing.T) {
	g, _ := BuildGraph(orYAML)
	const base = "pipeline.stages.s.spec.execution.steps."

	if res := g.Resolve(base + "onfail"); res.Join != JoinOR {
		t.Fatalf("failure-branch step Join = %v, want JoinOR", res.Join)
	}

	// OR: any ancestor having run is enough.
	if v, _ := g.EvaluateStep(base+"onfail", WithRan(ranSet(base+"main"))); !v.Allowed || v.Join != JoinOR {
		t.Errorf("OR with ancestor run should be allowed: %+v", v)
	}
	v, _ := g.EvaluateStep(base + "onfail")
	if v.Allowed {
		t.Errorf("OR with nothing run should be denied: %+v", v)
	}
	if !sortedEq(missingAncestors(v), []string{base + "main"}) {
		t.Errorf("MissingAncestors = %v, want [%s]", missingAncestors(v), base+"main")
	}
}

func TestVerifyStepOrder(t *testing.T) {
	const base = "pipeline.stages.build.spec.execution.steps."

	if v, err := VerifyStepOrder(graphYAML, base+"s1", nil); err != nil || !v.Allowed {
		t.Errorf("entry step via VerifyStepOrder should be allowed: %+v (err %v)", v, err)
	}
	if _, err := VerifyStepOrder("not: [valid: yaml:", base+"s1", nil); err == nil {
		t.Errorf("invalid YAML should yield an error")
	}
}
