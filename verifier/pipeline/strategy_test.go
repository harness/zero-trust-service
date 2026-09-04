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
	"testing"
)

// stepFQN builds the logical FQN of a step directly under st1's execution.
func stepFQN(id string) string {
	return "pipeline.stages.st1.spec.execution.steps." + id
}

// runSet returns a StepRanFunc backed by an exact-match set of concrete instance FQNs.
func runSet(fqns ...string) StepRanFunc {
	set := make(map[string]struct{}, len(fqns))
	for _, f := range fqns {
		set[f] = struct{}{}
	}
	return func(fqn string) (bool, error) { _, ok := set[fqn]; return ok, nil }
}

// TestStrategyEnumeration checks a strategy-bearing predecessor expands into the exact runtime instance FQNs across all looping modes, exclusions, nesting, and the non-enumerable runtime-input case.
func TestStrategyEnumeration(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		target        string
		wantAncFQN    string // logical FQN of the single ancestor
		wantExpands   bool
		wantInstances []string // nil when not statically enumerable
	}{
		{
			name:          "repeat times:3 -> _0.._2",
			file:          "case25_repeat_fanin.yaml",
			target:        stepFQN("after"),
			wantAncFQN:    stepFQN("deploy"),
			wantExpands:   true,
			wantInstances: []string{stepFQN("deploy_0"), stepFQN("deploy_1"), stepFQN("deploy_2")},
		},
		{
			name:        "matrix 2x2 -> positional row-major indices",
			file:        "case26_matrix_fanin.yaml",
			target:      stepFQN("after"),
			wantAncFQN:  stepFQN("build"),
			wantExpands: true,
			wantInstances: []string{
				stepFQN("build_0_0"), stepFQN("build_0_1"),
				stepFQN("build_1_0"), stepFQN("build_1_1"),
			},
		},
		{
			name:        "matrix exclude drops combo without renumbering",
			file:        "case27_matrix_exclude.yaml",
			target:      stepFQN("after"),
			wantAncFQN:  stepFQN("build"),
			wantExpands: true,
			// _0_0 (a:x,b:p) excluded; survivors keep original indices.
			wantInstances: []string{
				stepFQN("build_0_1"), stepFQN("build_1_0"), stepFQN("build_1_1"),
			},
		},
		{
			name:        "nested stepGroup(parallelism 2) x step(repeat 2) = 4",
			file:        "case28_nested_strategy.yaml",
			target:      stepFQN("after"),
			wantAncFQN:  stepFQN("sg.steps.g1"),
			wantExpands: true,
			wantInstances: []string{
				stepFQN("sg_0.steps.g1_0"), stepFQN("sg_0.steps.g1_1"),
				stepFQN("sg_1.steps.g1_0"), stepFQN("sg_1.steps.g1_1"),
			},
		},
		{
			name:          "parallelism: <+input> is a runtime fan-out, not enumerable",
			file:          "case29_runtime_parallelism.yaml",
			target:        stepFQN("after"),
			wantAncFQN:    stepFQN("deploy"),
			wantExpands:   true,
			wantInstances: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := BuildGraph(readFixture(t, tc.file))
			if err != nil || g == nil {
				t.Fatalf("BuildGraph returned (%v, %v) for %s", g, err, tc.file)
			}
			res := g.Resolve(tc.target)
			if !res.Found {
				t.Fatalf("target %s not found", tc.target)
			}
			if len(res.Ancestors) != 1 {
				t.Fatalf("expected 1 ancestor, got %d: %+v", len(res.Ancestors), res.Ancestors)
			}
			a := res.Ancestors[0]
			if a.FQN != tc.wantAncFQN {
				t.Errorf("ancestor FQN = %q, want %q", a.FQN, tc.wantAncFQN)
			}
			if a.Expands != tc.wantExpands {
				t.Errorf("Expands = %v, want %v", a.Expands, tc.wantExpands)
			}
			if !sameSet(a.Instances, tc.wantInstances) {
				t.Errorf("Instances = %v, want %v", a.Instances, tc.wantInstances)
			}
		})
	}
}

// TestMatrixValueNaming checks value-based expansion (axes sorted by key, values sanitized) under WithMatrixNaming(Value), and positional expansion by default.
func TestMatrixValueNaming(t *testing.T) {
	yaml := readFixture(t, "case30_matrix_value_naming.yaml")
	target := stepFQN("after")

	t.Run("value naming: sorted-by-key, sanitized values", func(t *testing.T) {
		g, err := BuildGraph(yaml, WithMatrixNaming(MatrixNamingValue))
		if err != nil || g == nil {
			t.Fatalf("BuildGraph returned (%v, %v)", g, err)
		}
		res := g.Resolve(target)
		if len(res.Ancestors) != 1 {
			t.Fatalf("expected 1 ancestor, got %d", len(res.Ancestors))
		}
		// Keys sort cloud,region so region varies fastest despite being declared first; "us-1" -> "us_1".
		want := []string{
			stepFQN("build_aws_us_1"), stepFQN("build_aws_eu"),
			stepFQN("build_gcp_us_1"), stepFQN("build_gcp_eu"),
		}
		if !sameSet(res.Ancestors[0].Instances, want) {
			t.Errorf("Instances = %v, want %v", res.Ancestors[0].Instances, want)
		}
	})

	t.Run("index naming (default): positional indices", func(t *testing.T) {
		g, _ := BuildGraph(yaml) // default = index
		res := g.Resolve(target)
		want := []string{
			stepFQN("build_0_0"), stepFQN("build_0_1"),
			stepFQN("build_1_0"), stepFQN("build_1_1"),
		}
		if !sameSet(res.Ancestors[0].Instances, want) {
			t.Errorf("Instances = %v, want %v", res.Ancestors[0].Instances, want)
		}
	})

	t.Run("resolving a matrix step by its instance FQN finds the logical node", func(t *testing.T) {
		// A verify request arriving with the instance FQN must resolve, not report not-found.
		gv, _ := BuildGraph(yaml, WithMatrixNaming(MatrixNamingValue))
		if res := gv.Resolve(stepFQN("build_aws_us_1")); !res.Found || res.FQN != stepFQN("build") {
			t.Errorf("value: Resolve(build_aws_us_1) = {Found:%v FQN:%q}, want {true build}", res.Found, res.FQN)
		}
		gi, _ := BuildGraph(yaml) // index
		if res := gi.Resolve(stepFQN("build_1_0")); !res.Found || res.FQN != stepFQN("build") {
			t.Errorf("index: Resolve(build_1_0) = {Found:%v FQN:%q}, want {true build}", res.Found, res.FQN)
		}
	})

	t.Run("value-mode fan-in gates on value FQNs", func(t *testing.T) {
		all := []string{
			stepFQN("build_aws_us_1"), stepFQN("build_aws_eu"),
			stepFQN("build_gcp_us_1"), stepFQN("build_gcp_eu"),
		}
		opt := []BuildOption{WithMatrixNaming(MatrixNamingValue)}
		if v, _ := VerifyStepOrder(yaml, target, opt, WithRan(runSet(all[:3]...))); v.Allowed {
			t.Fatalf("after should be denied at 3/4: %+v", v)
		}
		if v, _ := VerifyStepOrder(yaml, target, opt, WithRan(runSet(all...))); !v.Allowed {
			t.Fatalf("after should be allowed at 4/4: %+v", v)
		}
	})
}

// TestStrategyFanInGating verifies the AND fan-in gate: denied until every enumerated instance ran, and the missing set names the outstanding instances.
func TestStrategyFanInGating(t *testing.T) {
	after := stepFQN("after")

	t.Run("repeat: denied at 2 of 3, allowed at 3 of 3", func(t *testing.T) {
		yaml := readFixture(t, "case25_repeat_fanin.yaml")

		v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet(stepFQN("deploy_0"), stepFQN("deploy_1"))))
		if v.Allowed {
			t.Fatalf("after should be denied at 2/3: %+v", v)
		}
		if !sameSet(missingAncestors(v), []string{stepFQN("deploy_2")}) {
			t.Errorf("MissingAncestors = %v, want [deploy_2]", missingAncestors(v))
		}

		v, _ = VerifyStepOrder(yaml, after, nil,
			WithRan(runSet(stepFQN("deploy_0"), stepFQN("deploy_1"), stepFQN("deploy_2"))))
		if !v.Allowed {
			t.Fatalf("after should be allowed at 3/3: %+v", v)
		}
	})

	t.Run("matrix: denied at 3 of 4, allowed at 4 of 4", func(t *testing.T) {
		yaml := readFixture(t, "case26_matrix_fanin.yaml")
		all := []string{
			stepFQN("build_0_0"), stepFQN("build_0_1"),
			stepFQN("build_1_0"), stepFQN("build_1_1"),
		}

		v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet(all[0], all[1], all[2])))
		if v.Allowed {
			t.Fatalf("after should be denied at 3/4: %+v", v)
		}
		if !sameSet(missingAncestors(v), []string{stepFQN("build_1_1")}) {
			t.Errorf("MissingAncestors = %v, want [build_1_1]", missingAncestors(v))
		}

		v, _ = VerifyStepOrder(yaml, after, nil, WithRan(runSet(all...)))
		if !v.Allowed {
			t.Fatalf("after should be allowed at 4/4: %+v", v)
		}
	})

	t.Run("matrix exclude: allowed once the 3 survivors run", func(t *testing.T) {
		yaml := readFixture(t, "case27_matrix_exclude.yaml")
		survivors := []string{stepFQN("build_0_1"), stepFQN("build_1_0"), stepFQN("build_1_1")}
		if v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet(survivors...))); !v.Allowed {
			t.Fatalf("after should be allowed once all survivors ran: %+v", v)
		}
	})

	t.Run("nested composition: all four instances required", func(t *testing.T) {
		yaml := readFixture(t, "case28_nested_strategy.yaml")
		all := []string{
			stepFQN("sg_0.steps.g1_0"), stepFQN("sg_0.steps.g1_1"),
			stepFQN("sg_1.steps.g1_0"), stepFQN("sg_1.steps.g1_1"),
		}
		if v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet(all[:3]...))); v.Allowed {
			t.Fatalf("after should be denied until all 4 nested instances ran: %+v", v)
		}
		if v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet(all...))); !v.Allowed {
			t.Fatalf("after should be allowed once all 4 nested instances ran: %+v", v)
		}
	})

	t.Run("runtime-input fan-out is a RuntimeFanoutViolation", func(t *testing.T) {
		yaml := readFixture(t, "case29_runtime_parallelism.yaml")
		// A runtime-<+input> fan-out can't be gated statically, so it becomes its
		// own violation (fail-closed by default) rather than a missing ancestor.
		v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet()))
		if v.Allowed {
			t.Fatalf("expected denial on a non-enumerable runtime-input ancestor: %+v", v)
		}
		if !sameSet(runtimeFanouts(v), []string{stepFQN("deploy")}) {
			t.Errorf("RuntimeFanouts = %v, want [deploy]", runtimeFanouts(v))
		}
		if len(missingAncestors(v)) != 0 {
			t.Errorf("Missing = %v, want empty (not gated as a missing ancestor)", missingAncestors(v))
		}
		// A policy accepting runtime fan-out allows the step.
		failOpen := WithRuntimeFanoutPolicy(func(*RuntimeFanoutViolation) bool { return true })
		if v, _ := VerifyStepOrder(yaml, after, nil, WithRan(runSet()), failOpen); !v.Allowed {
			t.Fatalf("runtime-fanout policy should allow after: %+v", v)
		}
	})
}

// TestMatrixAllExcluded covers a matrix whose exclude list removes every combination:
// the engine skips the step gracefully (no instances run, pipeline does not fail), so
// the SDK must (a) surface the evaluated step as a ConditionViolation of kind
// strategy_excluded — defined not to run, fail-closed by default but relaxable — and
// (b) step the skipped node over as an ancestor, never gating a follower on a step
// that will never run. Exercised under both index and value naming, since the
// all-excluded fold happens on separate code paths.
func TestMatrixAllExcluded(t *testing.T) {
	build := stepFQN("build")
	after := stepFQN("after")

	for _, naming := range []struct {
		name string
		opts []BuildOption
	}{
		{"index naming", nil},
		{"value naming", []BuildOption{WithMatrixNaming(MatrixNamingValue)}},
	} {
		t.Run(naming.name, func(t *testing.T) {
			y := readFixture(t, "case31_matrix_all_excluded.yaml")

			// The strategy resolves to an excluded node (zero instances), not a build error.
			g, err := BuildGraph(y, naming.opts...)
			if err != nil || g == nil {
				t.Fatalf("BuildGraph returned (%v, %v), want a graph (all-excluded is a skip, not an error)", g, err)
			}
			if res := g.Resolve(build); !res.Found || !res.Excluded {
				t.Fatalf("build should resolve as Excluded, got Found=%v Excluded=%v", res.Found, res.Excluded)
			}

			// (a) Evaluating the excluded step itself: a strategy_excluded ConditionViolation,
			// fail-closed by default.
			v, _ := VerifyStepOrder(y, build, naming.opts, WithRan(runSet()))
			if v.Allowed {
				t.Fatalf("build should be denied by default (defined not to run): %+v", v)
			}
			cv := firstCondition(v)
			if cv == nil {
				t.Fatalf("expected a ConditionViolation on build, got %+v", v.Violations)
			}
			if cv.Kind != ConditionStrategyExcluded {
				t.Errorf("Kind = %v, want %v", cv.Kind, ConditionStrategyExcluded)
			}
			if cv.GuardFQN != build {
				t.Errorf("GuardFQN = %q, want %q", cv.GuardFQN, build)
			}

			// A policy accepting the strategy_excluded fact lets it through.
			acceptExcluded := WithConditionPolicy(func(c *ConditionViolation) bool {
				return c.Kind == ConditionStrategyExcluded
			})
			if vv, _ := VerifyStepOrder(y, build, naming.opts, WithRan(runSet()), acceptExcluded); !vv.Allowed {
				t.Fatalf("strategy_excluded policy should allow build: %+v", vv)
			}

			// (b) The follower is not gated on the skipped ancestor: build is stepped
			// over, and since it was after's only predecessor, after runs as an entry
			// step even though nothing has run.
			vAfter, _ := VerifyStepOrder(y, after, naming.opts, WithRan(runSet()))
			if !vAfter.Allowed {
				t.Fatalf("after should be allowed: its only ancestor (build) is skipped, not required: %+v", vAfter)
			}
			if fanouts := runtimeFanouts(vAfter); len(fanouts) != 0 {
				t.Errorf("after should have no runtime-fanout violations, got %v", fanouts)
			}
			if miss := missingAncestors(vAfter); len(miss) != 0 {
				t.Errorf("after should wait on nothing (skipped ancestor stepped over), got %v", miss)
			}
		})
	}
}

// emptyAxisMatrixYAML: s2 declares a matrix with a single EMPTY axis (`a: []`),
// which yields zero instances — the step never runs, just like an all-excluded
// matrix. after gates on s2 (AND).
const emptyAxisMatrixYAML = `
pipeline:
  identifier: pemptyaxis
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s2
                  type: ShellScript
                  strategy:
                    matrix:
                      a: []
              - step:
                  identifier: after
                  type: ShellScript
`

// noAxesMatrixYAML: a matrix with no axes at all (only maxConcurrency) — nothing
// to enumerate.
const noAxesMatrixYAML = `
pipeline:
  identifier: pnoaxes
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s2
                  type: ShellScript
                  strategy:
                    matrix:
                      maxConcurrency: 2
`

// TestMatrixNoInstances covers the "leaves no steps" matrix shapes other than
// exclude-all (see TestMatrixAllExcluded):
//
//   - no axes at all -> BuildGraph fails closed with ErrInvalidPipeline (nothing to
//     enumerate; a malformed strategy, not an intentional skip).
//   - an empty axis (`a: []`) -> a zero-instance matrix that leaves no steps. The
//     engine skips such a node gracefully (behind a Harness FF), so the SDK treats
//     it exactly like an all-excluded matrix: the node is `excluded`, its follower
//     steps over it, and the step itself fails closed but is relaxable via
//     WithConditionPolicy — the caller decides whether the skip is acceptable.
func TestMatrixNoInstances(t *testing.T) {
	t.Run("no axes is an invalid pipeline", func(t *testing.T) {
		if _, err := BuildGraph(noAxesMatrixYAML); err == nil {
			t.Fatal("a matrix with no axes should fail BuildGraph, got nil error")
		}
	})

	t.Run("empty axis is an excluded (skipped) node like all-excluded", func(t *testing.T) {
		s2, after := stepFQN("s2"), stepFQN("after")
		g, err := BuildGraph(emptyAxisMatrixYAML)
		if err != nil || g == nil {
			t.Fatalf("BuildGraph returned (%v, %v), want a graph (empty axis is not a build error)", g, err)
		}
		// The zero-instance step resolves as excluded — the fact that drives the
		// graceful step-over.
		if res := g.Resolve(s2); !res.Found || !res.Excluded {
			t.Fatalf("s2 should resolve Found and Excluded, got Found=%v Excluded=%v", res.Found, res.Excluded)
		}

		// Run 1 (no policy): evaluating the skipped step fails closed with a
		// strategy_excluded ConditionViolation.
		v, _ := g.EvaluateStep(s2, WithRan(ranSet()))
		if v.Allowed {
			t.Fatalf("s2 should be denied by default (defined not to run): %+v", v)
		}
		if cv := firstCondition(v); cv == nil || cv.Kind != ConditionStrategyExcluded {
			t.Fatalf("expected a strategy_excluded ConditionViolation on s2, got %+v", v.Violations)
		}

		// Run 2 (with policy): a caller that accepts the graceful skip lets it through.
		acceptExcluded := WithConditionPolicy(func(c *ConditionViolation) bool {
			return c.Kind == ConditionStrategyExcluded
		})
		if vv, _ := g.EvaluateStep(s2, WithRan(ranSet()), acceptExcluded); !vv.Allowed {
			t.Fatalf("strategy_excluded policy should allow s2: %+v", vv)
		}

		// The follower is not gated on the skipped ancestor: s2 is stepped over, so
		// after runs as an entry step even though nothing has run.
		vAfter, _ := g.EvaluateStep(after, WithRan(ranSet()))
		if !vAfter.Allowed {
			t.Fatalf("after should be allowed: its only ancestor (s2) is skipped, not required: %+v", vAfter)
		}
		if miss := missingAncestors(vAfter); len(miss) != 0 {
			t.Errorf("after should wait on nothing (skipped ancestor stepped over), got %v", miss)
		}
	})
}

// firstCondition returns the first ConditionViolation on the verdict (nil if none).
func firstCondition(v Verdict) *ConditionViolation {
	for _, vi := range v.Violations {
		if c := AsConditionViolation(vi); c != nil {
			return c
		}
	}
	return nil
}

// firstAncestorDidNotRun / firstRuntimeFanout return the first violation of that kind
// on the verdict (nil if none), so a test can assert its structured fields directly.
func firstAncestorDidNotRun(v Verdict) *AncestorDidNotRunViolation {
	for _, vi := range v.Violations {
		if m := AsAncestorDidNotRunViolation(vi); m != nil {
			return m
		}
	}
	return nil
}

func firstRuntimeFanout(v Verdict) *RuntimeFanoutViolation {
	for _, vi := range v.Violations {
		if r := AsRuntimeFanoutViolation(vi); r != nil {
			return r
		}
	}
	return nil
}

// TestViolationGranularity locks in the structured detail that lets a policy decide
// from fields rather than by parsing FQNs: AncestorDidNotRunViolation names the
// logical ancestor (AncestorFQN) and lists the still-outstanding instances
// (Ancestors), and RuntimeFanoutViolation distinguishes the evaluated step's own
// fan-out (FanoutSelf) from a dependency's (FanoutAncestor).
func TestViolationGranularity(t *testing.T) {
	t.Run("AncestorDidNotRun carries logical node and outstanding instances", func(t *testing.T) {
		// matrix 2x2: 4 instances of "build", 3 have run -> 1 outstanding.
		yaml := readFixture(t, "case26_matrix_fanin.yaml")
		ran := runSet(stepFQN("build_0_0"), stepFQN("build_0_1"), stepFQN("build_1_0"))
		v, _ := VerifyStepOrder(yaml, stepFQN("after"), nil, WithRan(ran))
		m := firstAncestorDidNotRun(v)
		if m == nil {
			t.Fatalf("expected an AncestorDidNotRunViolation, got %+v", v.Violations)
		}
		if m.AncestorFQN != stepFQN("build") {
			t.Errorf("AncestorFQN = %q, want %q", m.AncestorFQN, stepFQN("build"))
		}
		if !sameSet(m.Ancestors, []string{stepFQN("build_1_1")}) {
			t.Errorf("Ancestors = %v, want [build_1_1]", m.Ancestors)
		}
	})

	t.Run("self fan-out is FanoutSelf on the evaluated step", func(t *testing.T) {
		// "deploy" is an entry step with runtime-input parallelism: evaluating it
		// reports its OWN fan-out, fail-closed by default.
		yaml := readFixture(t, "case29_runtime_parallelism.yaml")
		v, _ := VerifyStepOrder(yaml, stepFQN("deploy"), nil, WithRan(runSet()))
		r := firstRuntimeFanout(v)
		if r == nil {
			t.Fatalf("expected a RuntimeFanoutViolation on deploy, got %+v", v.Violations)
		}
		if r.Role != FanoutSelf {
			t.Errorf("Role = %v, want %v", r.Role, FanoutSelf)
		}
		if r.FQN != stepFQN("deploy") {
			t.Errorf("FQN = %q, want %q", r.FQN, stepFQN("deploy"))
		}
		// A self-only policy lets the step run; the concept demo from the ordering example.
		selfOK := WithRuntimeFanoutPolicy(func(rf *RuntimeFanoutViolation) bool { return rf.Role == FanoutSelf })
		if vv, _ := VerifyStepOrder(yaml, stepFQN("deploy"), nil, WithRan(runSet()), selfOK); !vv.Allowed {
			t.Fatalf("self-fanout policy should allow deploy to run: %+v", vv)
		}
	})

	t.Run("ancestor fan-out is FanoutAncestor on the follower", func(t *testing.T) {
		yaml := readFixture(t, "case29_runtime_parallelism.yaml")
		v, _ := VerifyStepOrder(yaml, stepFQN("after"), nil, WithRan(runSet()))
		r := firstRuntimeFanout(v)
		if r == nil {
			t.Fatalf("expected a RuntimeFanoutViolation on after, got %+v", v.Violations)
		}
		if r.Role != FanoutAncestor {
			t.Errorf("Role = %v, want %v", r.Role, FanoutAncestor)
		}
		// The self-only policy must NOT accept a dependency's fan-out: after stays denied.
		selfOK := WithRuntimeFanoutPolicy(func(rf *RuntimeFanoutViolation) bool { return rf.Role == FanoutSelf })
		if vv, _ := VerifyStepOrder(yaml, stepFQN("after"), nil, WithRan(runSet()), selfOK); vv.Allowed {
			t.Fatalf("self-only policy must not allow after on an ancestor fan-out: %+v", vv)
		}
	})
}
