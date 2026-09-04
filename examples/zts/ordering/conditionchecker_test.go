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

// TestConditionChecker_DelegatesExpression verifies the checker hands the raw
// expression to the Evaluator and passes its verdict through, and that Vars are
// forwarded verbatim.
func TestConditionChecker_DelegatesExpression(t *testing.T) {
	var gotExpr string
	var gotVars map[string]string
	eval := ConditionEvaluatorFunc(func(_, expr string, vars map[string]string) (bool, bool) {
		gotExpr, gotVars = expr, vars
		return vars["run"] == "yes", true // stands in for a real JEXL engine
	})

	c := NewConditionChecker(eval, map[string]string{"run": "yes"})
	when := pipeline.When{Expression: `<+run> == "yes"`}
	if hold, _ := c.Eval("e", when); !hold {
		t.Error("expected Eval true when evaluator returns true")
	}
	if gotExpr != `<+run> == "yes"` {
		t.Errorf("evaluator got expr %q, want the raw when expression", gotExpr)
	}
	if gotVars["run"] != "yes" {
		t.Errorf("Vars not forwarded to evaluator: %v", gotVars)
	}

	// Override the value: same expression, evaluator now sees run=no -> false.
	c2 := NewConditionChecker(eval, map[string]string{"run": "no"})
	if hold, _ := c2.Eval("e", when); hold {
		t.Error("expected Eval false when overridden var makes the evaluator return false")
	}
}

// TestConditionChecker_Unresolvable checks that every unresolvable guard fails
// closed: a runtime-input when, a nil evaluator, and an evaluator that declines
// (known=false) all report hold=false and a non-nil ErrUnresolvedCondition. There
// is no fail-open opt-out.
func TestConditionChecker_Unresolvable(t *testing.T) {
	declines := ConditionEvaluatorFunc(func(_, _ string, _ map[string]string) (bool, bool) {
		return false, false // declines to evaluate (e.g. unsupported operator)
	})

	cases := []struct {
		name string
		chk  *ConditionChecker
		when pipeline.When
	}{
		{"runtime-input when", NewConditionChecker(declines, nil), pipeline.When{RuntimeInput: true}},
		{"nil evaluator with expression", NewConditionChecker(nil, nil), pipeline.When{Expression: `<+x> == "1"`}},
		{"evaluator declines", NewConditionChecker(declines, nil), pipeline.When{Expression: `<+a> > 1`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if hold, err := tc.chk.Eval("e", tc.when); hold || err == nil {
				t.Error("an unresolvable guard must fail closed: hold=false, err=ErrUnresolvedCondition")
			}
		})
	}
}

// TestConditionChecker_Overrides verifies an exact expression override wins
// outright, ahead of (and without needing) any evaluator, and that quotes in
// the expression are matched literally.
func TestConditionChecker_Overrides(t *testing.T) {
	const forceSkip = `<+pipeline.variables.name> == "override-me"`
	evalCalled := false
	eval := ConditionEvaluatorFunc(func(_, _ string, _ map[string]string) (bool, bool) {
		evalCalled = true
		return true, true // evaluator would allow; the override must win first
	})
	c := NewConditionChecker(eval, nil)
	c.Overrides = map[string]bool{forceSkip: false}

	if hold, err := c.Eval("e", pipeline.When{Expression: forceSkip}); hold || err != nil {
		t.Error("override to false should force skip (known, no error) regardless of the evaluator")
	}
	if evalCalled {
		t.Error("evaluator must not be consulted when an override matches")
	}

	// A no-override expression still falls through to the evaluator.
	if hold, _ := c.Eval("e", pipeline.When{Expression: `<+other> == "1"`}); !hold {
		t.Error("non-overridden expression should be evaluated (evaluator returns true)")
	}
}

// TestConditionChecker_MergeOverrides verifies config-supplied overrides overlay
// the checker's own Overrides, taking precedence.
func TestConditionChecker_MergeOverrides(t *testing.T) {
	c := NewConditionChecker(nil, nil)
	c.Overrides = map[string]bool{"a": true, "b": true}
	c.MergeOverrides(map[string]bool{"b": false, "c": true})

	if c.Overrides["a"] != true || c.Overrides["b"] != false || c.Overrides["c"] != true {
		t.Errorf("merged Overrides = %v, want a=true b=false c=true", c.Overrides)
	}

	c.MergeOverrides(nil)
	if len(c.Overrides) != 3 {
		t.Errorf("nil merge changed Overrides: %v", c.Overrides)
	}
}

func TestConditionChecker_StatusGate(t *testing.T) {
	const stage = "pipeline.stages.st1"
	c := NewConditionChecker(nil, nil)
	success := pipeline.When{Level: pipeline.WhenStep, OwnerFQN: stage, Status: "Success"}

	// "All"/"" always hold regardless of observed status.
	if hold, _ := c.Eval("e", pipeline.When{OwnerFQN: stage, Status: "All"}); !hold {
		t.Error(`Status "All" should always hold`)
	}
	// Success gate holds once the owner is observed successful.
	c.MarkStatus("e", stage, "Success")
	if hold, _ := c.Eval("e", success); !hold {
		t.Error("Success gate should hold when stage observed Success")
	}
	// ...and does not hold when the owner failed.
	c.MarkStatus("e", stage, "Failure")
	if hold, _ := c.Eval("e", success); hold {
		t.Error("Success gate must not hold when stage observed Failure")
	}
}
