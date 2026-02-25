package types

import "encoding/json"

// GitDetails contains git repository information for the pipeline YAML source
// Only present for remote/git-backed pipelines (null for inline pipelines)
type GitDetails struct {
	RepoName     string `json:"repoName,omitempty"`
	Branch       string `json:"branch,omitempty"`
	CommitID     string `json:"commitId,omitempty"`
	ConnectorRef string `json:"connectorRef,omitempty"`
	FilePath     string `json:"filePath,omitempty"`
}

// ExecutionDetails contains pipeline execution context (only present for pipeline-originated tasks)
type ExecutionDetails struct {
	PipelineExecutionID string `json:"pipelineExecutionId,omitempty"`
	StageExecutionID    string `json:"stageExecutionId,omitempty"`
	StepExecutionID     string `json:"stepExecutionId,omitempty"`
	PipelineIdentifier  string `json:"pipelineIdentifier,omitempty"`
}

// ZTSMetadata contains protected metadata set by the pipeline service
type ZTSMetadata struct {
	AccountID          string            `json:"accountId,omitempty"`
	OrgIdentifier      string            `json:"orgIdentifier,omitempty"`
	ProjectIdentifier  string            `json:"projectIdentifier,omitempty"`
	StepFQN            string            `json:"stepFqn,omitempty"`
	PipelineGitDetails *GitDetails       `json:"pipelineGitDetails,omitempty"`
	ExecutionDetails   *ExecutionDetails `json:"executionDetails,omitempty"`
	ParentUniqueID     string            `json:"parentUniqueId,omitempty"`
}

// TaskDetails contains the delegate task execution details.
// Field names match the DelegateTaskPackage "data" / "taskDataV2" JSON structure.
type TaskDetails struct {
	Parked                 bool              `json:"parked,omitempty"`
	Async                  bool              `json:"async,omitempty"`
	TaskType               string            `json:"taskType,omitempty"`
	Parameters             json.RawMessage   `json:"parameters,omitempty"`
	Timeout                int64             `json:"timeout,omitempty"`
	ExpressionFunctorToken int               `json:"expressionFunctorToken,omitempty"`
	Expressions            map[string]string `json:"expressions,omitempty"`
	SerializationFormat    string            `json:"serializationFormat,omitempty"`
	Data                   json.RawMessage   `json:"data,omitempty"`
}

// TaskSelector represents a delegate selector
type TaskSelector struct {
	Selector string `json:"selector,omitempty"`
	Origin   string `json:"origin,omitempty"`
}
