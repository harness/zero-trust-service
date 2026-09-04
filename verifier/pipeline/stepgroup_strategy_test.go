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

// grpInst builds a runtime instance FQN inside case33's fanned step group: grpInst(0, "g1") -> ...steps.grp_0.steps.g1.
func grpInst(copyIdx int, step string) string {
	suf := []string{"_0", "_1", "_2"}[copyIdx]
	return "pipeline.stages.st1.spec.execution.steps.grp" + suf + ".steps." + step
}

// TestStepGroupStrategyInternalDependency: within a step group fanned out 3x, each g2 instance must gate only on its own-branch g1, not all three (the over-constraint bug falsely serialized parallel copies).
func TestStepGroupStrategyInternalDependency(t *testing.T) {
	yaml := readFixture(t, "case33_stepgroup_strategy_internal.yaml")
	g, err := BuildGraph(yaml)
	if err != nil || g == nil {
		t.Fatalf("BuildGraph returned (%v, %v)", g, err)
	}
	g1Logical := "pipeline.stages.st1.spec.execution.steps.grp.steps.g1"

	t.Run("each g2 instance gates only on its own-branch g1", func(t *testing.T) {
		for k := range 3 {
			res := g.Resolve(grpInst(k, "g2"))
			if !res.Found {
				t.Fatalf("copy %d: g2 instance not found", k)
			}
			if len(res.Ancestors) != 1 {
				t.Fatalf("copy %d: expected 1 ancestor, got %+v", k, res.Ancestors)
			}
			a := res.Ancestors[0]
			if a.FQN != g1Logical {
				t.Errorf("copy %d: ancestor FQN = %q, want %q", k, a.FQN, g1Logical)
			}
			if !a.Expands {
				t.Errorf("copy %d: ancestor should expand", k)
			}
			if !sameSet(a.Instances, []string{grpInst(k, "g1")}) {
				t.Errorf("copy %d: Instances = %v, want [%s] only (no cross-branch g1)",
					k, a.Instances, grpInst(k, "g1"))
			}
		}
	})

	t.Run("g2 allowed once its own g1 ran, regardless of sibling branches", func(t *testing.T) {
		// Only grp_0's g1 ran: grp_0.g2 allowed (branch complete), grp_1.g2 still denied.
		ran := runSet(grpInst(0, "g1"))
		if v, _ := g.EvaluateStep(grpInst(0, "g2"), WithRan(ran)); !v.Allowed {
			t.Fatalf("grp_0.g2 should be allowed once grp_0.g1 ran: %+v", v)
		}
		if v, _ := g.EvaluateStep(grpInst(1, "g2"), WithRan(ran)); v.Allowed {
			t.Fatalf("grp_1.g2 must stay denied while grp_1.g1 has not run: %+v", v)
		}
	})

	t.Run("g2 does not wait on cross-branch g1 completion", func(t *testing.T) {
		// Other branches' g1 running must not satisfy grp_0.g2 — the gate is its own branch, not any g1.
		ran := runSet(grpInst(1, "g1"), grpInst(2, "g1"))
		if v, _ := g.EvaluateStep(grpInst(0, "g2"), WithRan(ran)); v.Allowed {
			t.Fatalf("grp_0.g2 must not be satisfied by other branches' g1: %+v", v)
		}
	})
}
