package pipeline

import "testing"

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
