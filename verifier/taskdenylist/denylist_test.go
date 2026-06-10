package taskdenylist

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestDenylist_DelegateAndGitops(t *testing.T) {
	v, err := New(Config{DeniedTypes: []string{"GITOPS_APP_SYNC", "SHELL_SCRIPT_TASK_NG"}})
	if err != nil {
		t.Fatal(err)
	}

	gitops := types.VerifyRequest{TaskPackage: &types.TaskPackage{
		GitOpsAgentID: "agent-1",
		TaskDetails:   &types.TaskDetails{TaskType: "GITOPS_APP_SYNC"},
	}}
	if err := v.Handle(context.Background(), gitops); err == nil {
		t.Fatal("expected gitops sync denied")
	}

	delegate := types.VerifyRequest{TaskPackage: &types.TaskPackage{
		TaskDetails: &types.TaskDetails{TaskType: "SHELL_SCRIPT_TASK_NG"},
	}}
	if err := v.Handle(context.Background(), delegate); err == nil {
		t.Fatal("expected shell script denied")
	}

	allowed := types.VerifyRequest{TaskPackage: &types.TaskPackage{
		GitOpsAgentID: "agent-1",
		TaskDetails: &types.TaskDetails{TaskType: "GITOPS_APP_GET"},
	}}
	if err := v.Handle(context.Background(), allowed); err != nil {
		t.Fatalf("expected app get allowed: %v", err)
	}
}

func TestDenylist_EmptyConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for empty denied_types")
	}
}
