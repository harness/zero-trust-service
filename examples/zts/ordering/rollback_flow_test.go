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

	"github.com/harness/zero-trust-service/resolver"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
)

// tc10PipelineRollbackYAML: a forward step (deploy) plus rollbackSteps
// (rb_deploy -> rb_deploy2) under a PipelineRollback failure action. Rollback runs
// in a SEPARATE plan, so the forward run's state is invisible during rollback verification.
const tc10PipelineRollbackYAML = `
pipeline:
  identifier: zts_tc10_pipeline_rollback
  stages:
    - stage:
        identifier: st1
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: deploy
                  type: ShellScript
            rollbackSteps:
              - step:
                  identifier: rb_deploy
                  type: ShellScript
              - step:
                  identifier: rb_deploy2
                  type: ShellScript
        failureStrategies:
          - onFailure:
              errors:
                - AllErrors
              action:
                type: PipelineRollback
`

// rbReq builds a verify request for a rollback step in the given execution.
func rbReq(exec, stepFQN string) (context.Context, types.VerifyRequest) {
	ctx, h := verifier.WithPipelineHolder(context.Background())
	h.Set(&resolver.ResolvedPipeline{ResolvedYAML: tc10PipelineRollbackYAML})
	req := types.VerifyRequest{TaskPackage: &types.TaskPackage{
		ZTSMetadata: &types.ZTSMetadata{
			StepFQN:          stepFQN,
			ExecutionDetails: &types.ExecutionDetails{PipelineExecutionID: exec},
		},
	}}
	return ctx, req
}

// TestOrderingVerifier_PipelineRollbackFailsOpen: PipelineRollback runs rb_deploy in
// a SEPARATE plan with no link to the forward run, so the entry must fail open rather
// than deny on the invisible forward run-state; ordering for that plan is out of scope.
func TestOrderingVerifier_PipelineRollbackFailsOpen(t *testing.T) {
	v, err := New(Config{}) // default: DenyOnMissing=true
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const forwardExec = "OlNgWrUxSCGDcdps3YvFpQ"
	const rollbackExec = "R06oaGbHRr2bH365yfl7wg" // the separate rollback plan
	rbDeploy := "pipeline.stages.st1.spec.execution.rollbackSteps.rb_deploy"
	rbDeploy2 := "pipeline.stages.st1.spec.execution.rollbackSteps.rb_deploy2"

	// Forward step runs, recorded under the pipeline's own execution.
	ctxFwd, reqFwd := rbReq(forwardExec, "pipeline.stages.st1.spec.execution.steps.deploy")
	if err := v.Handle(ctxFwd, reqFwd); err != nil {
		t.Fatalf("forward deploy should be allowed (entry step), got: %v", err)
	}

	// Rollback entry runs in a separate plan that never saw the forward step; it
	// fails open but is still recorded so later rollback steps can gate on it.
	ctxE, reqE := rbReq(rollbackExec, rbDeploy)
	if err := v.Handle(ctxE, reqE); err != nil {
		t.Fatalf("rb_deploy (entry) should fail open under PipelineRollback, got deny: %v", err)
	}

	// Later rollback step gates on its predecessor within the same plan, which just ran.
	ctx2, req2 := rbReq(rollbackExec, rbDeploy2)
	if err := v.Handle(ctx2, req2); err != nil {
		t.Fatalf("rb_deploy2 should be allowed after rb_deploy ran in the same plan, got: %v", err)
	}
}

// TestOrderingVerifier_PipelineRollbackLaterStepStillGated: fail-open applies only to
// the entry; a later rollback step is denied if its predecessor has not run in the plan.
func TestOrderingVerifier_PipelineRollbackLaterStepStillGated(t *testing.T) {
	v, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Fresh rollback plan; rb_deploy has not run in it.
	ctx, req := rbReq("s9bI-RULR3ial9vbwaPlTA", "pipeline.stages.st1.spec.execution.rollbackSteps.rb_deploy2")
	if err := v.Handle(ctx, req); err == nil {
		t.Fatal("rb_deploy2 must be denied until rb_deploy runs in the same rollback plan")
	}
}
