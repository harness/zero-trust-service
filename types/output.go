package types

import (
	"context"
	"encoding/json"
)

// OutputRequest is sent by the delegate or GitOps agent after task completion.
type OutputRequest struct {
	TaskID       string              `json:"taskId,omitempty"`
	TaskResponse *TaskOutputResponse `json:"taskResponse"`
}

// TaskOutputResponse contains the task result metadata. GitOps agents
// populate GitOpsAgentID; delegates leave it empty.
type TaskOutputResponse struct {
	AccountID    string          `json:"accountId,omitempty"`
	TaskTypeName string          `json:"taskTypeName,omitempty"`
	ResponseCode string          `json:"responseCode,omitempty"`
	GitOpsAgentID string         `json:"gitOpsAgentId,omitempty"`
	Response     json.RawMessage `json:"response,omitempty"`
}

type OutputResponse struct {
	Error string `json:"error,omitempty"`
}

// OutputHandler is the function signature for handling output requests.
type OutputHandler func(ctx context.Context, request OutputRequest) (OutputResponse, error)

// AccountID returns the account ID from the task response, or empty if
// the response is missing.
func (r OutputRequest) AccountID() string {
	if r.TaskResponse == nil {
		return ""
	}
	return r.TaskResponse.AccountID
}

// TaskTypeName returns the task type name from the task response, or
// empty if the response is missing.
func (r OutputRequest) TaskTypeName() string {
	if r.TaskResponse == nil {
		return ""
	}
	return r.TaskResponse.TaskTypeName
}

// ResponseCode returns the response code from the task response, or
// empty if the response is missing.
func (r OutputRequest) ResponseCode() string {
	if r.TaskResponse == nil {
		return ""
	}
	return r.TaskResponse.ResponseCode
}

// GitOpsAgentID returns the GitOps agent ID, or empty for delegate tasks.
func (r OutputRequest) GitOpsAgentID() string {
	if r.TaskResponse == nil {
		return ""
	}
	return r.TaskResponse.GitOpsAgentID
}
