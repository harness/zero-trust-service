package pipeline

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

const regularPipeline = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: deploy
        type: Deployment
        spec:
          execution:
            steps:
              - step:
                  identifier: shell1
                  type: ShellScript
`

const ciPipeline = `
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
                  identifier: run1
                  type: Run
                  spec:
                    command: echo hello
`

const containerisedStepGroupPipeline = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: deploy
        type: Custom
        spec:
          execution:
            steps:
              - stepGroup:
                  identifier: sg1
                  spec:
                    infrastructure:
                      type: KubernetesDirect
                  steps:
                    - step:
                        identifier: run1
                        type: ShellScript
`

func TestLookupStep_Found(t *testing.T) {
	result := LookupStep(regularPipeline, "pipeline.stages.deploy.spec.execution.steps.shell1")
	if result.Status != StepFound {
		t.Fatalf("expected StepFound, got %v", result.Status)
	}
	if result.StepType != "ShellScript" {
		t.Errorf("expected ShellScript, got %s", result.StepType)
	}
}

func TestLookupStep_MissingInRegularStage(t *testing.T) {
	result := LookupStep(regularPipeline, "pipeline.stages.deploy.spec.execution.steps.nonexistent")
	if result.Status != StepMissing {
		t.Fatalf("expected StepMissing, got %v", result.Status)
	}
}

func TestLookupStep_StageNotFound(t *testing.T) {
	result := LookupStep(regularPipeline, "pipeline.stages.nonexistent.spec.execution.steps.shell1")
	if result.Status != StepMissing {
		t.Fatalf("expected StepMissing, got %v", result.Status)
	}
}

func TestLookupStep_CIStageRealStepFound(t *testing.T) {
	result := LookupStep(ciPipeline, "pipeline.stages.build.spec.execution.steps.run1")
	if result.Status != StepFound {
		t.Fatalf("expected StepFound, got %v", result.Status)
	}
	if result.StepType != "Run" {
		t.Errorf("expected Run, got %s", result.StepType)
	}
}

func TestLookupStep_CIStageInjectedStepNotFound(t *testing.T) {
	// Injected steps (liteEngineTask) are not in the YAML — they should be StepMissing.
	result := LookupStep(ciPipeline, "pipeline.stages.build.spec.execution.steps.liteEngineTask")
	if result.Status != StepMissing {
		t.Fatalf("expected StepMissing, got %v", result.Status)
	}
}

func TestLookupStep_ContainerisedStepGroupRealStepFound(t *testing.T) {
	result := LookupStep(containerisedStepGroupPipeline,
		"pipeline.stages.deploy.spec.execution.steps.sg1.steps.run1")
	if result.Status != StepFound {
		t.Fatalf("expected StepFound, got %v", result.Status)
	}
	if result.StepType != "ShellScript" {
		t.Errorf("expected ShellScript, got %s", result.StepType)
	}
}

func TestLookupStep_InvalidYAML(t *testing.T) {
	result := LookupStep("not: [valid: yaml:", "pipeline.stages.x")
	if result.Status != StepMissing {
		t.Fatalf("expected StepMissing for invalid YAML, got %v", result.Status)
	}
}

type execDetails struct{ pipelineExecID string }

// ctxWithPipeline returns a context with an empty PipelineHolder; set is
// unexported, so these tests exercise NewStepLookup's nil-pipeline fast path.
func ctxWithPipeline(_ string) context.Context {
	ctx, _ := verifier.WithPipelineHolder(context.Background())
	return ctx
}

func makeReq(accountID, stepFQN string, exec *execDetails) types.VerifyRequest {
	if accountID == "" && stepFQN == "" {
		return types.VerifyRequest{}
	}
	meta := &types.ZTSMetadata{AccountID: accountID, StepFQN: stepFQN}
	if exec != nil {
		meta.ExecutionDetails = &types.ExecutionDetails{PipelineExecutionID: exec.pipelineExecID}
	}
	return types.VerifyRequest{TaskPackage: &types.TaskPackage{ZTSMetadata: meta}}
}

func TestNewStepLookup(t *testing.T) {
	tests := []struct {
		name string
		cfg  StepLookupConfig
		ctx  context.Context
		req  types.VerifyRequest
	}{
		{"nil pipeline", StepLookupConfig{}, context.Background(), makeReq("acc1", "step1", nil)},
		{"nil task package", StepLookupConfig{}, ctxWithPipeline(regularPipeline), makeReq("", "", nil)},
		{"empty step fqn", StepLookupConfig{LogFound: true, LogMissing: true}, ctxWithPipeline(regularPipeline), makeReq("acc1", "", nil)},
		{"found logged", StepLookupConfig{LogFound: true}, ctxWithPipeline(regularPipeline), makeReq("acc1", "pipeline.stages.deploy.spec.execution.steps.shell1", nil)},
		{"missing logged", StepLookupConfig{LogMissing: true}, ctxWithPipeline(regularPipeline), makeReq("acc1", "pipeline.stages.deploy.spec.execution.steps.nonexistent", nil)},
		{"with execution id", StepLookupConfig{LogFound: true}, ctxWithPipeline(regularPipeline), makeReq("acc1", "pipeline.stages.deploy.spec.execution.steps.shell1", &execDetails{pipelineExecID: "exec-123"})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewStepLookup(tc.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := v.Handle(tc.ctx, tc.req); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
