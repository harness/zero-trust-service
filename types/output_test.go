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
