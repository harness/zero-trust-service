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

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/pipeline"
)

// orderingYAML: s1, then a parallel (p_a, p_b), then s2 joining on both.
const orderingYAML = `
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
`

// TestRanStore_DrivesVerify drives VerifyStepOrder through an AND fan-in and confirms per-execution isolation.
func TestRanStore_DrivesVerify(t *testing.T) {
	const base = "pipeline.stages.build.spec.execution.steps."
	store := NewStore()

	// s2 is an AND join on p_a & p_b: denied before both run.
	if v, _ := pipeline.VerifyStepOrder(orderingYAML, base+"s2", nil, pipeline.WithRan(func(f string) (bool, error) { return store.Ran("exec-1", f), nil })); v.Allowed {
		t.Fatalf("s2 should be denied before ancestors run: %+v", v)
	}
	store.MarkRan("exec-1", base+"p_a", base+"p_a")
	store.MarkRan("exec-1", base+"p_b", base+"p_b")
	if v, _ := pipeline.VerifyStepOrder(orderingYAML, base+"s2", nil, pipeline.WithRan(func(f string) (bool, error) { return store.Ran("exec-1", f), nil })); !v.Allowed {
		t.Fatalf("s2 should be allowed after both ancestors run: %+v", v)
	}

	// State is isolated per plan execution ID: exec-2 sees none of exec-1's runs.
	if v, _ := pipeline.VerifyStepOrder(orderingYAML, base+"s2", nil, pipeline.WithRan(func(f string) (bool, error) { return store.Ran("exec-2", f), nil })); v.Allowed {
		t.Fatalf("exec-2 shares no state with exec-1: %+v", v)
	}
}

func TestRanStore_LogicalAndInstanceQueries(t *testing.T) {
	store := NewStore()
	// A recorded instance satisfies its logical node, but a specific sibling
	// instance is matched exactly — this is what makes AND fan-in enforceable.
	store.MarkRan("e", "pipeline.stages.d.spec.execution.steps.deploy_0",
		"pipeline.stages.d.spec.execution.steps.deploy")
	if !store.Ran("e", "pipeline.stages.d.spec.execution.steps.deploy") {
		t.Error("instance should satisfy its logical FQN")
	}
	if !store.Ran("e", "pipeline.stages.d.spec.execution.steps.deploy_0") {
		t.Error("the instance that ran should match exactly")
	}
	if store.Ran("e", "pipeline.stages.d.spec.execution.steps.deploy_1") {
		t.Error("a sibling instance that did NOT run must not match (exact, not folded)")
	}
	if store.Ran("e", "pipeline.stages.d.spec.execution.steps.other") {
		t.Error("unrelated FQN must not match")
	}
}

func TestRanStore_ValueNamedInstanceFoldsToLogical(t *testing.T) {
	store := NewStore()
	// Value-named instance with a digit-ending step id ("ShellScript_1"): folding is
	// exact (caller supplies the logical FQN) and never corrupts the step id.
	inst := "pipeline.stages.cd_env1_cds_s2.spec.execution.steps.ShellScript_1"
	logical := "pipeline.stages.cd.spec.execution.steps.ShellScript_1"
	store.MarkRan("e", inst, logical)
	if !store.Ran("e", logical) {
		t.Error("value-named instance should satisfy its logical FQN")
	}
	if !store.Ran("e", inst) {
		t.Error("the value-named instance that ran should match exactly")
	}
}

func TestRanStore_RetainsIndividualInstancesForAudit(t *testing.T) {
	store := NewStore()
	base := "pipeline.stages.st1.spec.execution.steps.s1"
	// All four matrix instances of s1 complete (each folds onto logical base).
	for _, suf := range []string{"_0_0", "_0_1", "_1_0", "_1_1"} {
		store.MarkRan("e", base+suf, base)
	}
	// Audit view keeps every instance verbatim (nothing folded away).
	got := store.Instances("e")
	want := []string{base + "_0_0", base + "_0_1", base + "_1_0", base + "_1_1"}
	if len(got) != len(want) {
		t.Fatalf("Instances = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Instances[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// The logical ancestor edge is still satisfied by the folded view.
	if !store.Ran("e", base) {
		t.Error("logical ancestor s1 should be satisfied by its instances")
	}
	// Isolation: another execution shares nothing.
	if len(store.Instances("other")) != 0 {
		t.Error("instances must be isolated per plan execution")
	}
}

func TestRanStore_FoldsMultiAxisMatrixSuffix(t *testing.T) {
	store := NewStore()
	// A matrix instance carries one "_<n>" suffix per axis (e.g. "run_1_0"); it folds
	// onto the logical node, but a specific sibling instance is still matched exactly.
	store.MarkRan("e", "pipeline.stages.d.spec.execution.steps.run_1_0",
		"pipeline.stages.d.spec.execution.steps.run")
	if !store.Ran("e", "pipeline.stages.d.spec.execution.steps.run") {
		t.Error("multi-axis instance suffix should fold onto the logical FQN")
	}
	if !store.Ran("e", "pipeline.stages.d.spec.execution.steps.run_1_0") {
		t.Error("the multi-axis instance that ran should match exactly")
	}
	if store.Ran("e", "pipeline.stages.d.spec.execution.steps.run_0_1") {
		t.Error("a sibling multi-axis instance that did NOT run must not match")
	}
}
