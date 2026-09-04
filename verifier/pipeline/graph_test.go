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
	"strings"
	"testing"
)

// graphYAML exercises sequential steps, a parallel fan-out/join, a step group with its own `when`, cross-stage ordering, stage/step guards, and a strategy-bearing step.
const graphYAML = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: build
        type: CI
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: Run
              - parallel:
                  - step:
                      identifier: p_a
                      type: Run
                  - step:
                      identifier: p_b
                      type: Run
              - step:
                  identifier: s2
                  type: Run
                  when:
                    stageStatus: Success
                    condition: "<+foo> == \"bar\""
    - stage:
        identifier: deploy
        type: Deployment
        when:
          pipelineStatus: Success
        spec:
          execution:
            steps:
              - stepGroup:
                  identifier: sg1
                  when:
                    stageStatus: All
                  steps:
                    - step:
                        identifier: g1
                        type: Http
                    - step:
                        identifier: g2
                        type: Http
              - step:
                  identifier: matrixStep
                  type: ShellScript
                  strategy:
                    matrix:
                      env: [dev, prod]
`

func fqns(as []Ancestor) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.FQN
	}
	sort.Strings(out)
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildGraph_NotNil(t *testing.T) {
	if g, err := BuildGraph(graphYAML); g == nil || err != nil {
		t.Fatalf("BuildGraph returned (%v, %v), want graph and no error", g, err)
	}
	if g, err := BuildGraph("not: [valid: yaml:"); g != nil || !errors.Is(err, ErrInvalidPipeline) {
		t.Errorf("invalid YAML: got (%v, %v), want (nil, ErrInvalidPipeline)", g, err)
	}
	if g, err := BuildGraph("notPipeline: {}"); g != nil || !errors.Is(err, ErrInvalidPipeline) {
		t.Errorf("no pipeline root: got (%v, %v), want (nil, ErrInvalidPipeline)", g, err)
	}
}

func TestResolve_Ancestors(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const base = "pipeline.stages.build.spec.execution.steps."
	const dep = "pipeline.stages.deploy.spec.execution.steps."

	tests := []struct {
		name string
		fqn  string
		want []string
	}{
		{"first step of first stage has no ancestors", base + "s1", nil},
		{"parallel members share the pre-block predecessor", base + "p_a", []string{base + "s1"}},
		{"other parallel member same predecessor", base + "p_b", []string{base + "s1"}},
		{"step after parallel joins on all branches", base + "s2", []string{base + "p_a", base + "p_b"}},
		{"first step of group inherits group's predecessor across stage boundary",
			dep + "sg1.steps.g1", []string{base + "s2"}},
		{"second step in group follows its sibling", dep + "sg1.steps.g2", []string{dep + "sg1.steps.g1"}},
		{"step after group joins on group's terminal", dep + "matrixStep", []string{dep + "sg1.steps.g2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := g.Resolve(tt.fqn)
			if !res.Found {
				t.Fatalf("step not found: %s", tt.fqn)
			}
			got := fqns(res.Ancestors)
			if !eq(got, tt.want) {
				t.Errorf("ancestors = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolve_ConditionsAtBoundaryOnly(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const dep = "pipeline.stages.deploy.spec.execution.steps."

	// g1 is first in group sg1, first in stage deploy: it carries the group's `when` (All) and stage's `when` (Success), with no own guard.
	res := g.Resolve(dep + "sg1.steps.g1")
	levels := map[WhenLevel]WhenStatus{}
	for _, w := range res.Conditions {
		levels[w.Level] = w.Status
	}
	if levels[WhenStepGroup] != "All" {
		t.Errorf("g1 missing stepGroup guard, got conditions %+v", res.Conditions)
	}
	if levels[WhenStage] != "Success" {
		t.Errorf("g1 missing stage guard, got conditions %+v", res.Conditions)
	}

	// g2 follows a sibling, so container guards are already implied — it carries NO conditions.
	res2 := g.Resolve(dep + "sg1.steps.g2")
	if len(res2.Conditions) != 0 {
		t.Errorf("g2 should have no conditions, got %+v", res2.Conditions)
	}

	// Ancestors carry their step type: g1's predecessor s2 is a Run step.
	if a := g.Resolve(dep + "matrixStep").Ancestors; len(a) != 1 || a[0].Type != "Http" {
		t.Errorf("expected one Http ancestor for matrixStep, got %+v", a)
	}

	// s2 has its own `when` with a JEXL condition.
	res3 := g.Resolve("pipeline.stages.build.spec.execution.steps.s2")
	if len(res3.Conditions) != 1 || res3.Conditions[0].Level != WhenStep {
		t.Fatalf("s2 conditions = %+v", res3.Conditions)
	}
	if res3.Conditions[0].Expression == "" || res3.Conditions[0].Status != "Success" {
		t.Errorf("s2 when not parsed: %+v", res3.Conditions[0])
	}
}

func TestResolve_StrategyNormalization(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	const dep = "pipeline.stages.deploy.spec.execution.steps."

	// Query with a runtime strategy suffix; should normalize to the logical node.
	res := g.Resolve(dep + "matrixStep_1")
	if !res.Found {
		t.Fatal("matrixStep_1 not found after normalization")
	}
	if res.FQN != dep+"matrixStep" {
		t.Errorf("FQN = %q, want logical %q", res.FQN, dep+"matrixStep")
	}
	if res.Type != "ShellScript" {
		t.Errorf("Type = %q, want ShellScript", res.Type)
	}
	if !res.HasStrategy {
		t.Error("matrixStep should report HasStrategy")
	}

	// A pool entry with a strategy suffix should match its logical ancestor.
	a := Ancestor{FQN: dep + "matrixStep", Expands: true}
	if !g.MatchesAncestor(dep+"matrixStep_0", a) {
		t.Error("MatchesAncestor should match runtime instance against logical ancestor")
	}
	if g.MatchesAncestor(dep+"other", a) {
		t.Error("MatchesAncestor should not match a different step")
	}
}

func TestResolve_SuffixOnNonStrategyStepFails(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	// s1 has no strategy, so a "_0" suffix must NOT silently resolve to s1.
	res := g.Resolve("pipeline.stages.build.spec.execution.steps.s1_0")
	if res.Found {
		t.Error("suffix on a non-strategy step should not resolve")
	}
}

func TestDuplicates(t *testing.T) {
	const dupYAML = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: s
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: dup
                  type: Run
              - step:
                  identifier: dup
                  type: Run
`
	// Duplicate identifiers make the pipeline invalid: BuildGraph fails closed.
	g, err := BuildGraph(dupYAML)
	if g != nil {
		t.Errorf("expected nil graph on duplicate identifiers, got %+v", g)
	}
	if !errors.Is(err, ErrInvalidPipeline) {
		t.Fatalf("expected ErrInvalidPipeline, got %v", err)
	}
	if !strings.Contains(err.Error(), "pipeline.stages.s.spec.execution.steps.dup") {
		t.Errorf("error should name the duplicate FQN, got %v", err)
	}
}

func TestBuildGraph_MalformedWhenFailsClosed(t *testing.T) {
	// `when` given as a sequence is neither an object nor a runtime expression:
	// the pipeline is malformed and BuildGraph must fail rather than drop the guard.
	const badWhenYAML = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: s
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s1
                  type: Run
                  when:
                    - stageStatus: Success
`
	g, err := BuildGraph(badWhenYAML)
	if g != nil {
		t.Errorf("expected nil graph on malformed when, got %+v", g)
	}
	if !errors.Is(err, ErrInvalidPipeline) {
		t.Fatalf("expected ErrInvalidPipeline, got %v", err)
	}
}

func TestResolve_Missing(t *testing.T) {
	g, _ := BuildGraph(graphYAML)
	res := g.Resolve("pipeline.stages.build.spec.execution.steps.nope")
	if res.Found {
		t.Error("expected not found")
	}
}

func TestContainsExpression(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"<+input>", true},
		{"<+input>.default(dev)", true},
		{"<+input>.executionInput()", true},
		{"<+input>.allowedValues(a,b)", true},
		{"<+pipeline.variables.flag>", true}, // variable ref, not <+input>
		{"<+INPUT>", true},                   // case: "<+" has no letters, still matched
		{"  <+input>  ", true},               // surrounding whitespace
		{"echo <+step.name> done", true},     // expression embedded mid-string
		{"Success", false},
		{"", false},
		{"deploy-prod", false},
		{"<+input", false},    // opener without a closer is not an expression
		{"a > b <+ c", false}, // closer precedes the opener: no delimited token
	}
	for _, tt := range tests {
		if got := containsExpression(tt.in); got != tt.want {
			t.Errorf("containsExpression(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestRawWhen_AnyExpressionIsRuntimeInput: a scalar `when` that is any Harness
// expression (not just the literal "<+input>") must surface RuntimeInput=true so
// it is never enforced statically.
func TestRawWhen_AnyExpressionIsRuntimeInput(t *testing.T) {
	const tmpl = `
pipeline:
  identifier: p
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
                  when: %q
`
	for _, expr := range []string{"<+input>", "<+pipeline.variables.flag>", "<+INPUT>", "<+input>.executionInput()"} {
		g, err := BuildGraph(fmt.Sprintf(tmpl, expr))
		if err != nil {
			t.Fatalf("BuildGraph(%q): %v", expr, err)
		}
		res := g.Resolve("pipeline.stages.st1.spec.execution.steps.s2")
		if len(res.Conditions) != 1 || !res.Conditions[0].RuntimeInput {
			t.Errorf("when %q: conditions = %+v, want single RuntimeInput guard", expr, res.Conditions)
		}
	}
}
