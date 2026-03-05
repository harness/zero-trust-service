package types

import "encoding/json"

type OutputRequest struct {
	TaskID       string              `json:"taskId,omitempty"`
	TaskResponse *TaskOutputResponse `json:"taskResponse"`
}

type TaskOutputResponse struct {
	AccountID    string          `json:"accountId,omitempty"`
	TaskTypeName string          `json:"taskTypeName,omitempty"`
	ResponseCode string          `json:"responseCode,omitempty"`
	Response     json.RawMessage `json:"response,omitempty"`
}

type OutputResponse struct {
	Error string `json:"error,omitempty"`
}
