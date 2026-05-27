package types

import "encoding/json"

// DelegateTaskPackage mirrors the Java DelegateTaskPackage structure.
// Field names match the JSON serialization from the Java side exactly.
type DelegateTaskPackage struct {
	TaskID                string                 `json:"delegateTaskId"`
	AccountID             string                 `json:"accountId,omitempty"`
	DelegateID            string                 `json:"delegateId,omitempty"`
	DelegateInstanceID    string                 `json:"delegateInstanceId,omitempty"`
	RunnerResponse        bool                   `json:"runnerResponse,omitempty"`
	TaskDetails           *TaskDetails           `json:"data,omitempty"`
	TaskDataV2            *TaskDetails           `json:"taskDataV2,omitempty"`
	EncryptionConfigs     map[string]interface{} `json:"encryptionConfigs,omitempty"`
	SecretDetails         map[string]interface{} `json:"secretDetails,omitempty"`
	Secrets               []string               `json:"secrets,omitempty"`
	ExecutionCapabilities []interface{}          `json:"executionCapabilities,omitempty"`
	LogAbstractions       map[string]string      `json:"logStreamingAbstractions,omitempty"`
	ShouldSkipOpenStream  bool                   `json:"shouldSkipOpenStream,omitempty"`
	BaseLogKey            string                 `json:"baseLogKey,omitempty"`
	ZTSMetadata           *ZTSMetadata           `json:"ztsMetadata,omitempty"`
	IsAborted             bool                   `json:"isAborted,omitempty"`
}

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

// ResolveAccountID returns the account ID, preferring ZTSMetadata but falling
// back to the top-level accountId from the DelegateTaskPackage.
func (r VerifyRequest) ResolveAccountID() string {
	if r.TaskPackage == nil {
		return ""
	}
	if r.TaskPackage.ZTSMetadata != nil && r.TaskPackage.ZTSMetadata.AccountID != "" {
		return r.TaskPackage.ZTSMetadata.AccountID
	}
	return r.TaskPackage.AccountID
}

// ResolveTaskType returns the task type from the task package data.
func (r VerifyRequest) ResolveTaskType() string {
	if r.TaskPackage == nil || r.TaskPackage.TaskDetails == nil {
		return ""
	}
	return r.TaskPackage.TaskDetails.TaskType
}

// TaskID returns the delegate task ID, or empty string if the task package
// is missing.
func (r VerifyRequest) TaskID() string {
	if r.TaskPackage == nil {
		return ""
	}
	return r.TaskPackage.TaskID
}

// DelegateID returns the delegate ID from the task package, or empty if
// the task package is missing.
func (r VerifyRequest) DelegateID() string {
	if r.TaskPackage == nil {
		return ""
	}
	return r.TaskPackage.DelegateID
}

// DelegateInstanceID returns the delegate instance ID from the task
// package, or empty if the task package is missing.
func (r VerifyRequest) DelegateInstanceID() string {
	if r.TaskPackage == nil {
		return ""
	}
	return r.TaskPackage.DelegateInstanceID
}
