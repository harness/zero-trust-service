package pipeline

import (
	"context"
	"log"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// StepLookupConfig holds the step lookup verifier configuration.
type StepLookupConfig struct {
	LogFound   bool `yaml:"log_found"`
	LogMissing bool `yaml:"log_missing"`
}

// NewStepLookup creates a step lookup validator from typed config.
func NewStepLookup(cfg StepLookupConfig) (verifier.Interface, error) {
	logFound := cfg.LogFound
	logMissing := cfg.LogMissing

	return verifier.From(func(ctx context.Context, request types.VerifyRequest) error {
		rp := verifier.ResolvedPipelineFrom(ctx)
		if rp == nil || rp.ResolvedYAML == "" {
			return nil
		}

		pkg := request.TaskPackage
		if pkg == nil || pkg.ZTSMetadata == nil {
			return nil
		}
		zts := pkg.ZTSMetadata
		stepFQN := zts.StepFQN
		if stepFQN == "" {
			return nil
		}

		accountID := request.ResolveAccountID()
		taskType := request.ResolveTaskType()
		taskID := request.TaskID()
		executionID := ""
		if zts.ExecutionDetails != nil {
			executionID = zts.ExecutionDetails.PipelineExecutionID
		}

		result := LookupStep(rp.ResolvedYAML, stepFQN)

		switch result.Status {
		case StepFound:
			if logFound {
				log.Printf("[step_lookup] found step fqn=%s type=%s account=%s taskType=%s taskId=%s execution=%s",
					stepFQN, result.StepType, accountID, taskType, taskID, executionID)
			}
		case StepMissing:
			if logMissing {
				log.Printf("[step_lookup] step not found fqn=%s account=%s taskType=%s taskId=%s execution=%s",
					stepFQN, accountID, taskType, taskID, executionID)
			}
		}

		return nil
	}), nil
}

// StepStatus indicates whether a step was found or missing.
type StepStatus int

const (
	StepFound   StepStatus = iota
	StepMissing
)

// LookupResult contains the outcome of a step lookup.
type LookupResult struct {
	Status   StepStatus
	StepType string
}

// LookupStep searches for a step in the given pipeline YAML using its FQN.
func LookupStep(pipelineYAML, fqn string) LookupResult {
	root := ParsePipeline(pipelineYAML)
	if root == nil {
		return LookupResult{Status: StepMissing}
	}

	node := FindNodeByFQN(root, fqn)
	if node != nil {
		return LookupResult{Status: StepFound, StepType: GetNodeType(node)}
	}

	return LookupResult{Status: StepMissing}
}
