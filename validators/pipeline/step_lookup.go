package pipeline

import (
	"context"
	"log"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// StepLookup creates a validator that looks up the current step in the
// resolved pipeline YAML using the stepFqn from ZTSMetadata.
//
// Config keys (all optional):
//
//	log_found: bool   — log when a step is found (default: true)
//	log_missing: bool — log when a step is not found (default: true)
func StepLookup(cfg map[string]any) (verifier.Interface, error) {
	logFound := boolCfg(cfg, "log_found", true)
	logMissing := boolCfg(cfg, "log_missing", true)

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
		taskID := pkg.TaskID
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
	StepMissing            // step not found in the resolved YAML
)

// LookupResult contains the outcome of a step lookup.
type LookupResult struct {
	Status   StepStatus
	StepType string // populated when Status == StepFound
}

// LookupStep searches for a step in the given pipeline YAML using its FQN.
// It returns whether the step was found or missing.
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

func boolCfg(cfg map[string]any, key string, defaultVal bool) bool {
	if cfg == nil {
		return defaultVal
	}
	v, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}
