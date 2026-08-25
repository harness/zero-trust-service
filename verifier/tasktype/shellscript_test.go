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

package tasktype

import (
	"context"
	"encoding/json"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func shellReq(params string) types.VerifyRequest {
	return types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "SHELL_SCRIPT_TASK_NG",
				Parameters: json.RawMessage(params),
			},
		},
	}
}

func bashValidator(t *testing.T, commands ...string) verifier.Interface {
	t.Helper()
	v, err := NewShellScript(ShellScriptConfig{Bash: commands})
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	return v
}

func TestShellScript_ConfigEmpty(t *testing.T) {
	_, err := NewShellScript(ShellScriptConfig{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestShellScript_NilTaskPackage(t *testing.T) {
	v := bashValidator(t, "echo")
	req := types.VerifyRequest{}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil TaskPackage: %v", err)
	}
}

func TestShellScript_NilTaskDetails(t *testing.T) {
	v := bashValidator(t, "echo")
	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil TaskDetails: %v", err)
	}
}

func TestShellScript_EmptyParameters(t *testing.T) {
	v := bashValidator(t, "echo")
	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "SHELL_SCRIPT_TASK_NG"},
		},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for empty parameters: %v", err)
	}
}

func TestShellScript_EmptyScriptType(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"","script":"rm -rf /"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when scriptType is empty (skipped): %v", err)
	}
}

func TestShellScript_EmptyScript(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":""}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for empty script: %v", err)
	}
}

func TestShellScript_SimpleAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"echo hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_MultipleAllowedCommands(t *testing.T) {
	v := bashValidator(t, "echo", "curl", "grep")
	req := shellReq(`[{"scriptType":"BASH","script":"echo hello && curl http://example.com | grep foo"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_PureAssignment(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"FOO=bar"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for pure assignment: %v", err)
	}
}

func TestShellScript_CommandNotAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"rm -rf /"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command")
	}
}

func TestShellScript_ScriptTypeNotConfigured(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"POWERSHELL","script":"echo hello"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for unconfigured script type")
	}
}

func TestShellScript_DynamicCommandName(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"$CMD hello"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for dynamic command name")
	}
}

func TestShellScript_ExportAllowed(t *testing.T) {
	v := bashValidator(t, "export")
	req := shellReq(`[{"scriptType":"BASH","script":"export FOO=bar"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when export is allowed: %v", err)
	}
}

func TestShellScript_ExportNotAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"export FOO=bar"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error when export is not in allowlist")
	}
}

func TestShellScript_InvalidJSON(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`not json`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for invalid JSON parameters")
	}
}

func TestShellScript_SyntaxError(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"if true; then echo"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for script with syntax error")
	}
}
