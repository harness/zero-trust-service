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
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestResolveAncestors asserts per-fixture ordering: ancestor set, join type, strategy flag, and surfaced when-conditions.
func TestResolveAncestors(t *testing.T) {
	// step builds the logical FQN of a step directly under a stage's execution.
	step := func(stage, id string) string {
		return "pipeline.stages." + stage + ".spec.execution.steps." + id
	}

	type condCheck struct {
		level        WhenLevel
		status       string
		hasExpr      bool
		runtimeInput bool
	}

	tests := []struct {
		name        string
		file        string
		target      string
		wantFound   bool
		wantAnc     []string // order-insensitive
		wantJoin    JoinType
		wantStrat   bool
		wantConds   []condCheck // subset match
		resolveFQN  string      // override: resolve this instead of target (runtime suffix cases)
		wantResolve string      // override: expected Resolution.FQN when resolveFQN set
	}{
		{
			name:      "case01 sequential steps: s2 waits on s1",
			file:      "case01_sequential_steps.yaml",
			target:    step("st1", "s2"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case02 sequential stages: first step of st2 waits on last step of st1",
			file:      "case02_sequential_stages.yaml",
			target:    step("st2", "b1"),
			wantFound: true,
			wantAnc:   []string{step("st1", "a1")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case03 parallel branch p_a waits on s1",
			file:      "case03_parallel_after_step.yaml",
			target:    step("st1", "p_a"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case03 parallel branch p_b waits on s1",
			file:      "case03_parallel_after_step.yaml",
			target:    step("st1", "p_b"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case04 step after parallel waits on all branches",
			file:      "case04_step_after_parallel.yaml",
			target:    step("st1", "s2"),
			wantFound: true,
			wantAnc:   []string{step("st1", "p_a"), step("st1", "p_b")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case05 first step of group inherits group's predecessor",
			file:      "case05_nested_step_groups.yaml",
			target:    "pipeline.stages.st1.spec.execution.steps.sg1.steps.g1a",
			wantFound: true,
			wantAnc:   []string{step("st1", "pre")},
			wantJoin:  JoinAND,
		},
		{
			name:      "case05 first step of nested group waits on prior sibling",
			file:      "case05_nested_step_groups.yaml",
			target:    "pipeline.stages.st1.spec.execution.steps.sg1.steps.sg2.steps.g2a",
			wantFound: true,
			wantAnc:   []string{"pipeline.stages.st1.spec.execution.steps.sg1.steps.g1a"},
			wantJoin:  JoinAND,
		},
		{
			name:      "case06 strategy step: logical node carries HasStrategy, ancestor is prior step",
			file:      "case06_strategy_step.yaml",
			target:    step("st1", "deploy"),
			wantFound: true,
			wantAnc:   []string{step("st1", "first")},
			wantJoin:  JoinAND,
			wantStrat: true,
		},
		{
			name:        "case06 runtime instance deploy_0 normalizes to logical deploy",
			file:        "case06_strategy_step.yaml",
			target:      step("st1", "deploy"),
			resolveFQN:  step("st1", "deploy") + "_0",
			wantResolve: step("st1", "deploy"),
			wantFound:   true,
			wantAnc:     []string{step("st1", "first")},
			wantJoin:    JoinAND,
			wantStrat:   true,
		},
		{
			name:        "case06 matrix instance deploy_1_0 (multi-axis) normalizes to logical deploy",
			file:        "case06_strategy_step.yaml",
			target:      step("st1", "deploy"),
			resolveFQN:  step("st1", "deploy") + "_1_0",
			wantResolve: step("st1", "deploy"),
			wantFound:   true,
			wantAnc:     []string{step("st1", "first")},
			wantJoin:    JoinAND,
			wantStrat:   true,
		},
		{
			name:      "case21 failure-branch step is an OR join",
			file:      "case21_failure_branch.yaml",
			target:    step("st1", "onfail"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinOR,
			wantConds: []condCheck{{level: WhenStep, status: "Failure"}},
		},
		{
			name:      "case22 step when: condition surfaced, ordering unchanged",
			file:      "case22_step_when_condition.yaml",
			target:    step("st1", "s2"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinAND,
			wantConds: []condCheck{{level: WhenStep, status: "Success", hasExpr: true}},
		},
		{
			name:      "case23 stage when surfaced on entry step of the stage",
			file:      "case23_stage_when_pipelinestatus.yaml",
			target:    step("st2", "b1"),
			wantFound: true,
			wantAnc:   []string{step("st1", "a1")},
			wantJoin:  JoinAND,
			wantConds: []condCheck{{level: WhenStage, status: "Success"}},
		},
		{
			name:      "case24 runtime-input when surfaced",
			file:      "case24_when_runtime_input.yaml",
			target:    step("st1", "s2"),
			wantFound: true,
			wantAnc:   []string{step("st1", "s1")},
			wantJoin:  JoinAND,
			wantConds: []condCheck{{level: WhenStep, runtimeInput: true}},
		},
		{
			name:      "missing FQN resolves as not found",
			file:      "case01_sequential_steps.yaml",
			target:    step("st1", "nope"),
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := BuildGraph(readFixture(t, tc.file))
			if err != nil || g == nil {
				t.Fatalf("BuildGraph returned (%v, %v) for %s", g, err, tc.file)
			}

			toResolve := tc.target
			if tc.resolveFQN != "" {
				toResolve = tc.resolveFQN
			}
			res := g.Resolve(toResolve)

			if res.Found != tc.wantFound {
				t.Fatalf("Found = %v, want %v (reason: %+v)", res.Found, tc.wantFound, res)
			}
			if !tc.wantFound {
				return
			}
			if tc.wantResolve != "" && res.FQN != tc.wantResolve {
				t.Errorf("resolved FQN = %q, want %q", res.FQN, tc.wantResolve)
			}
			if res.Join != tc.wantJoin {
				t.Errorf("Join = %v, want %v", res.Join, tc.wantJoin)
			}
			if res.HasStrategy != tc.wantStrat {
				t.Errorf("HasStrategy = %v, want %v", res.HasStrategy, tc.wantStrat)
			}

			gotAnc := make([]string, len(res.Ancestors))
			for i, a := range res.Ancestors {
				gotAnc[i] = a.FQN
			}
			if !sameSet(gotAnc, tc.wantAnc) {
				t.Errorf("ancestors = %v, want %v", gotAnc, tc.wantAnc)
			}

			for _, want := range tc.wantConds {
				if !hasCondition(res.Conditions, want) {
					t.Errorf("expected surfaced condition %+v in %+v", want, res.Conditions)
				}
			}
		})
	}
}

// TestEvaluateStepWithFixtures verifies the EvaluateStep gate: AND requires all ancestors, OR requires any.
func TestEvaluateStepWithFixtures(t *testing.T) {
	step := func(stage, id string) string {
		return "pipeline.stages." + stage + ".spec.execution.steps." + id
	}
	ranSet := func(fqns ...string) StepRanFunc {
		set := make(map[string]struct{}, len(fqns))
		for _, f := range fqns {
			set[f] = struct{}{}
		}
		return func(fqn string) (bool, error) { _, ok := set[fqn]; return ok, nil }
	}

	tests := []struct {
		name        string
		file        string
		target      string
		ran         StepRanFunc
		wantAllowed bool
		wantMissing []string
	}{
		{
			name:        "AND satisfied: s2 allowed once s1 has run",
			file:        "case01_sequential_steps.yaml",
			target:      step("st1", "s2"),
			ran:         ranSet(step("st1", "s1")),
			wantAllowed: true,
		},
		{
			name:        "AND unsatisfied: s2 denied when s1 has not run",
			file:        "case01_sequential_steps.yaml",
			target:      step("st1", "s2"),
			ran:         ranSet(),
			wantAllowed: false,
			wantMissing: []string{step("st1", "s1")},
		},
		{
			name:        "AND over parallel: s2 denied until both branches run",
			file:        "case04_step_after_parallel.yaml",
			target:      step("st1", "s2"),
			ran:         ranSet(step("st1", "p_a")),
			wantAllowed: false,
			wantMissing: []string{step("st1", "p_b")},
		},
		{
			name:        "AND over parallel: s2 allowed once both branches run",
			file:        "case04_step_after_parallel.yaml",
			target:      step("st1", "s2"),
			ran:         ranSet(step("st1", "p_a"), step("st1", "p_b")),
			wantAllowed: true,
		},
		{
			name:        "OR satisfied: failure-branch step allowed once s1 has run",
			file:        "case21_failure_branch.yaml",
			target:      step("st1", "onfail"),
			ran:         ranSet(step("st1", "s1")),
			wantAllowed: true,
		},
		{
			name:        "OR unsatisfied: failure-branch step denied when nothing ran",
			file:        "case21_failure_branch.yaml",
			target:      step("st1", "onfail"),
			ran:         ranSet(),
			wantAllowed: false,
			wantMissing: []string{step("st1", "s1")},
		},
		{
			name:        "entry step allowed with no ancestors",
			file:        "case01_sequential_steps.yaml",
			target:      step("st1", "s1"),
			ran:         ranSet(),
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, _ := VerifyStepOrder(readFixture(t, tc.file), tc.target, nil, WithRan(tc.ran))
			if v.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v (verdict: %+v)", v.Allowed, tc.wantAllowed, v)
			}
			if !sameSet(missingAncestors(v), tc.wantMissing) {
				t.Errorf("MissingAncestors = %v, want %v", missingAncestors(v), tc.wantMissing)
			}
		})
	}
}

func hasCondition(conds []When, want struct {
	level        WhenLevel
	status       string
	hasExpr      bool
	runtimeInput bool
}) bool {
	for _, w := range conds {
		if w.Level != want.level {
			continue
		}
		if w.Status != WhenStatus(want.status) {
			continue
		}
		if w.RuntimeInput != want.runtimeInput {
			continue
		}
		if want.hasExpr != (w.Expression != "") {
			continue
		}
		return true
	}
	return false
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
