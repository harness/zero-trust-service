package types

import (
	"encoding/json"
	"testing"
)

func TestResolveAccountID_PreferZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID:   "top-level",
			ZTSMetadata: &ZTSMetadata{AccountID: "zts-meta"},
		},
	}
	if got := req.ResolveAccountID(); got != "zts-meta" {
		t.Errorf("expected zts-meta, got %q", got)
	}
}

func TestResolveAccountID_FallbackToTopLevel(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID: "top-level",
		},
	}
	if got := req.ResolveAccountID(); got != "top-level" {
		t.Errorf("expected top-level, got %q", got)
	}
}

func TestResolveAccountID_EmptyZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			AccountID:   "top-level",
			ZTSMetadata: &ZTSMetadata{AccountID: ""},
		},
	}
	if got := req.ResolveAccountID(); got != "top-level" {
		t.Errorf("expected fallback to top-level, got %q", got)
	}
}

func TestResolveAccountID_AllEmpty(t *testing.T) {
	req := VerifyRequest{}
	if got := req.ResolveAccountID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveAccountID_NilTaskPackage(t *testing.T) {
	req := VerifyRequest{TaskPackage: nil}
	if got := req.ResolveAccountID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveTaskType(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &DelegateTaskPackage{
			TaskDetails: &TaskDetails{TaskType: "SHELL_SCRIPT"},
		},
	}
	if got := req.ResolveTaskType(); got != "SHELL_SCRIPT" {
		t.Errorf("expected SHELL_SCRIPT, got %q", got)
	}
}

func TestResolveTaskType_NilTaskPackage(t *testing.T) {
	req := VerifyRequest{}
	if got := req.ResolveTaskType(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveTaskType_NilTaskDetails(t *testing.T) {
	req := VerifyRequest{TaskPackage: &DelegateTaskPackage{}}
	if got := req.ResolveTaskType(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestVerifyRequest_UnmarshalPerpetualTaskGitPolling verifies that a GITPOLLING_NG perpetual task
// payload produced by PerpetualTaskZtsParamDecorator on the delegate is correctly parsed.
// Kryo bytes fields are decoded into plain JSON and sent inside taskPackage.data.parameters.
func TestVerifyRequest_UnmarshalPerpetualTaskGitPolling(t *testing.T) {
	body := []byte(`{"taskPackage":{"delegateTaskId":"task-1","accountId":"test-account","data":{"taskType":"GITPOLLING_NG","parameters":[{"pollingDocId":"polling-doc-1","gitpollingWebhookParams":{"accountId":"test-account","attributes":{"sourceType":"Github","webhookId":"123","connectorDetails":{"identifier":"test-connector","connectorConfig":{"url":"https://github.com/test-org/test-repo.git"}}},"gitPollingTaskType":"GET_WEBHOOK_EVENTS"},"_type":"io.harness.perpetualtask.polling.GitPollingTaskParamsNg"}]},"ztsMetadata":{"accountId":"test-account","orgIdentifier":"test-org","projectIdentifier":"test-project"}}}`)

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if req.TaskPackage == nil {
		t.Fatal("expected task package")
	}
	if req.TaskPackage.ZTSMetadata == nil {
		t.Fatal("expected zts metadata")
	}
	if got := req.TaskPackage.ZTSMetadata.AccountID; got != "test-account" {
		t.Errorf("zts metadata accountId: expected test-account, got %q", got)
	}
	if req.TaskPackage.TaskDetails == nil {
		t.Fatal("expected task details")
	}
	if got := req.TaskPackage.TaskDetails.TaskType; got != "GITPOLLING_NG" {
		t.Errorf("taskType: expected GITPOLLING_NG, got %q", got)
	}
	if len(req.TaskPackage.TaskDetails.Parameters) == 0 {
		t.Fatal("expected non-empty decoded parameters")
	}
	if got := req.ResolveAccountID(); got != "test-account" {
		t.Errorf("ResolveAccountID: expected test-account, got %q", got)
	}
	if got := req.ResolveTaskType(); got != "GITPOLLING_NG" {
		t.Errorf("ResolveTaskType: expected GITPOLLING_NG, got %q", got)
	}
}
