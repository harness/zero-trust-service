// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"
	"testing"
)

func TestResolveAccountID_PreferZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
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
		TaskPackage: &TaskPackage{
			AccountID: "top-level",
		},
	}
	if got := req.ResolveAccountID(); got != "top-level" {
		t.Errorf("expected top-level, got %q", got)
	}
}

func TestResolveAccountID_EmptyZTSMetadata(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
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
		TaskPackage: &TaskPackage{
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
	req := VerifyRequest{TaskPackage: &TaskPackage{}}
	if got := req.ResolveTaskType(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestVerifyRequest_UnmarshalPerpetualTaskGitPolling verifies that a GITPOLLING_NG perpetual task
// payload produced by PerpetualTaskZtsParamDecorator on the delegate is correctly parsed.
func TestVerifyRequest_UnmarshalPerpetualTaskGitPolling(t *testing.T) {
	body := []byte(`{"taskPackage":{"delegateTaskId":"task-1","accountId":"test-account","data":{"taskType":"GITPOLLING_NG","parameters":[{"pollingDocId":"polling-doc-1","gitpollingWebhookParams":{"accountId":"test-account","attributes":{"sourceType":"Github","webhookId":"123","connectorDetails":{"identifier":"test-connector","connectorConfig":{"url":"https://github.com/test-org/test-repo.git"}}}},"gitPollingTaskType":"GET_WEBHOOK_EVENTS","_type":"io.harness.perpetualtask.polling.GitPollingTaskParamsNg"}]},"ztsMetadata":{"accountId":"test-account","orgIdentifier":"test-org","projectIdentifier":"test-project"}}}`)

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

func TestResolveAccountID_GitopsTask(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
			AccountID: "gitops-account",
			GitOpsAgentID: "agent-1",
		},
	}
	if got := req.ResolveAccountID(); got != "gitops-account" {
		t.Errorf("expected gitops-account, got %q", got)
	}
}

func TestResolveTaskType_GitopsTask(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
			TaskDetails: &TaskDetails{TaskType: "GITOPS_APP_SYNC"},
			GitOpsAgentID: "agent-1",
		},
	}
	if got := req.ResolveTaskType(); got != "GITOPS_APP_SYNC" {
		t.Errorf("expected GITOPS_APP_SYNC, got %q", got)
	}
}

func TestGitOpsAgentID(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
			GitOpsAgentID: "my-agent",
		},
	}
	if got := req.GitOpsAgentID(); got != "my-agent" {
		t.Errorf("expected my-agent, got %q", got)
	}
}

func TestGitOpsAgentID_Empty(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{},
	}
	if got := req.GitOpsAgentID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestGitOpsAgentID_NilTaskPackage(t *testing.T) {
	req := VerifyRequest{}
	if got := req.GitOpsAgentID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestTaskID_Delegate(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
			TaskID: "delegate-task-123",
		},
	}
	if got := req.TaskID(); got != "delegate-task-123" {
		t.Errorf("expected delegate-task-123, got %q", got)
	}
}

func TestTaskID_Gitops(t *testing.T) {
	req := VerifyRequest{
		TaskPackage: &TaskPackage{
			TaskID:  "gitops-task-456",
			GitOpsAgentID: "agent-1",
		},
	}
	if got := req.TaskID(); got != "gitops-task-456" {
		t.Errorf("expected gitops-task-456, got %q", got)
	}
}

func TestTaskID_DelegatePrecedence(t *testing.T) {
	body := []byte(`{"taskPackage":{"delegateTaskId":"delegate-id","taskId":"gitops-id"}}`)
	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got := req.TaskID(); got != "delegate-id" {
		t.Errorf("expected delegateTaskId to take precedence, got %q", got)
	}
}

func TestTaskID_Empty(t *testing.T) {
	req := VerifyRequest{}
	if got := req.TaskID(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestUnmarshalDelegateTaskPackage verifies delegate wire format (delegateTaskId).
func TestUnmarshalDelegateTaskPackage(t *testing.T) {
	body := []byte(`{"taskPackage":{"delegateTaskId":"t1","accountId":"abc","data":{"taskType":"SHELL_SCRIPT_TASK_NG"}}}`)

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got := req.TaskID(); got != "t1" {
		t.Errorf("taskId: expected t1, got %q", got)
	}
	if got := req.GitOpsAgentID(); got != "" {
		t.Errorf("gitOpsAgentId: expected empty for delegate, got %q", got)
	}
}

// TestUnmarshalGitopsViaTaskPackage verifies GitOps wire format (taskId + gitOpsAgentId).
func TestUnmarshalGitopsViaTaskPackage(t *testing.T) {
	body := []byte(`{"taskPackage":{"taskId":"t1","accountId":"abc","gitOpsAgentId":"prod-agent","data":{"taskType":"GITOPS_APP_SYNC"}}}`)

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got := req.ResolveAccountID(); got != "abc" {
		t.Errorf("accountId: expected abc, got %q", got)
	}
	if got := req.ResolveTaskType(); got != "GITOPS_APP_SYNC" {
		t.Errorf("taskType: expected GITOPS_APP_SYNC, got %q", got)
	}
	if got := req.GitOpsAgentID(); got != "prod-agent" {
		t.Errorf("gitOpsAgentId: expected prod-agent, got %q", got)
	}
	if got := req.TaskID(); got != "t1" {
		t.Errorf("taskId: expected t1, got %q", got)
	}
}

func TestDelegateID(t *testing.T) {
	if got := (VerifyRequest{TaskPackage: &TaskPackage{DelegateID: "del-1"}}).DelegateID(); got != "del-1" {
		t.Errorf("DelegateID = %q, want del-1", got)
	}
	if got := (VerifyRequest{}).DelegateID(); got != "" {
		t.Errorf("DelegateID = %q, want empty", got)
	}
}

func TestDelegateInstanceID(t *testing.T) {
	if got := (VerifyRequest{TaskPackage: &TaskPackage{DelegateInstanceID: "inst-1"}}).DelegateInstanceID(); got != "inst-1" {
		t.Errorf("DelegateInstanceID = %q, want inst-1", got)
	}
	if got := (VerifyRequest{}).DelegateInstanceID(); got != "" {
		t.Errorf("DelegateInstanceID = %q, want empty", got)
	}
}
