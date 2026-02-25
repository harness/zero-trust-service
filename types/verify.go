package types

import "context"

// VerifyRequest is the top-level request sent by the delegate service.
// It wraps a DelegateTaskPackage inside the "taskPackage" field, matching
// the Java ZtsVerificationRequest DTO.
type VerifyRequest struct {
	TaskPackage *DelegateTaskPackage `json:"taskPackage"`
}

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

// VerifyResponse is returned to the delegate service.
// Fields match the Java ZtsVerificationResponse DTO:
//   - allowed:  true if the task is permitted, false if denied
//   - reason:   human-readable explanation (present when denied)
//   - metadata: optional key-value pairs for additional context
type VerifyResponse struct {
	Allowed  bool                   `json:"allowed"`
	Reason   string                 `json:"reason,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VerifyHandler is the function signature for handling verify requests.
// The context carries request-scoped data such as the validator tracker.
type VerifyHandler func(ctx context.Context, request VerifyRequest) (VerifyResponse, error)
