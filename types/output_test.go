package types

import "testing"

func TestOutputRequest_NilTaskResponse(t *testing.T) {
	r := OutputRequest{}
	if got := r.AccountID(); got != "" {
		t.Errorf("AccountID = %q, want empty", got)
	}
	if got := r.TaskTypeName(); got != "" {
		t.Errorf("TaskTypeName = %q, want empty", got)
	}
	if got := r.ResponseCode(); got != "" {
		t.Errorf("ResponseCode = %q, want empty", got)
	}
	if got := r.GitOpsAgentID(); got != "" {
		t.Errorf("GitOpsAgentID = %q, want empty", got)
	}
}

func TestOutputRequest_WithTaskResponse(t *testing.T) {
	r := OutputRequest{
		TaskResponse: &TaskOutputResponse{
			AccountID:     "acc1",
			TaskTypeName:  "SHELL_SCRIPT",
			ResponseCode:  "OK",
			GitOpsAgentID: "agent-1",
		},
	}
	if got := r.AccountID(); got != "acc1" {
		t.Errorf("AccountID = %q, want acc1", got)
	}
	if got := r.TaskTypeName(); got != "SHELL_SCRIPT" {
		t.Errorf("TaskTypeName = %q, want SHELL_SCRIPT", got)
	}
	if got := r.ResponseCode(); got != "OK" {
		t.Errorf("ResponseCode = %q, want OK", got)
	}
	if got := r.GitOpsAgentID(); got != "agent-1" {
		t.Errorf("GitOpsAgentID = %q, want agent-1", got)
	}
}
