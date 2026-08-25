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
