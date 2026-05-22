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

func TestVerifyRequest_UnmarshalDecodedPayload(t *testing.T) {
	body := []byte(`{
		"taskPackage": {"delegateTaskId": "task-1"},
		"decodedPayload": {
			"kind": "CI_EXECUTE_STEP",
			"source": "serializedStep",
			"payload": {"executionId": "runtime"},
			"error": ""
		}
	}`)

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if req.DecodedPayload == nil {
		t.Fatal("expected decoded payload")
	}
	if req.DecodedPayload.Kind != "CI_EXECUTE_STEP" {
		t.Fatalf("expected CI_EXECUTE_STEP, got %q", req.DecodedPayload.Kind)
	}
	if got := req.DecodedPayload.Payload["executionId"]; got != "runtime" {
		t.Fatalf("expected runtime execution id, got %v", got)
	}
}
