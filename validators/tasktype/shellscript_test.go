package tasktype

import (
	"context"
	"encoding/json"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// helper to build a VerifyRequest with the given parameters JSON.
func shellReq(params string) types.VerifyRequest {
	return types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "SHELL_SCRIPT_TASK_NG",
				Parameters: json.RawMessage(params),
			},
		},
	}
}

// helper to build a validator allowing the given commands for BASH.
func bashValidator(t *testing.T, commands ...string) *shellscript {
	t.Helper()
	cmds := make([]any, len(commands))
	for i, c := range commands {
		cmds[i] = c
	}
	v, err := ShellScript(map[string]any{"bash": cmds})
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	return v.(*shellscript)
}

// --- Config validation ---

func TestShellScript_ConfigEmpty(t *testing.T) {
	_, err := ShellScript(map[string]any{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestShellScript_ConfigUnsupportedType(t *testing.T) {
	_, err := ShellScript(map[string]any{
		"powershell": []any{"Get-Process"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported script type")
	}
}

func TestShellScript_ConfigNotAList(t *testing.T) {
	_, err := ShellScript(map[string]any{
		"bash": "not-a-list",
	})
	if err == nil {
		t.Fatal("expected error when value is not a list")
	}
}

func TestShellScript_ConfigNonStringItem(t *testing.T) {
	_, err := ShellScript(map[string]any{
		"bash": []any{123},
	})
	if err == nil {
		t.Fatal("expected error for non-string item")
	}
}

func TestShellScript_ConfigCaseInsensitive(t *testing.T) {
	v, err := ShellScript(map[string]any{
		"BASH": []any{"echo"},
	})
	if err != nil {
		t.Fatalf("expected config with uppercase key to work: %v", err)
	}

	req := shellReq(`[{"scriptType":"BASH","script":"echo hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

// --- Nil / empty handling ---

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
		TaskPackage: &types.DelegateTaskPackage{},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil TaskDetails: %v", err)
	}
}

func TestShellScript_EmptyParameters(t *testing.T) {
	v := bashValidator(t, "echo")
	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
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

// --- Allowed (sunny path) ---

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

func TestShellScript_AssignmentWithCommand(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"FOO=bar echo hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for assignment with allowed command: %v", err)
	}
}

func TestShellScript_IfWithAllowedCommands(t *testing.T) {
	v := bashValidator(t, "true", "echo")
	script := `if true; then echo "yes"; fi`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_ForLoopWithAllowedCommands(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `for i in 1 2 3; do echo $i; done`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_WhileLoopWithAllowedCommands(t *testing.T) {
	v := bashValidator(t, "true", "echo", "break")
	script := `while true; do echo loop; break; done`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_SubshellAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"(echo hello)"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_CommandSubstitutionInArgAllowed(t *testing.T) {
	v := bashValidator(t, "echo", "date")
	script := `echo "today is $(date)"`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_PipeAllowed(t *testing.T) {
	v := bashValidator(t, "echo", "grep")
	req := shellReq(`[{"scriptType":"BASH","script":"echo hello | grep hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_ExportAllowed(t *testing.T) {
	v := bashValidator(t, "export")
	req := shellReq(`[{"scriptType":"BASH","script":"export FOO=bar"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when export is allowed: %v", err)
	}
}

func TestShellScript_DeclareAllowed(t *testing.T) {
	v := bashValidator(t, "declare")
	req := shellReq(`[{"scriptType":"BASH","script":"declare -i x=42"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when declare is allowed: %v", err)
	}
}

func TestShellScript_LocalAllowed(t *testing.T) {
	v := bashValidator(t, "local")
	script := `local x=1`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when local is allowed: %v", err)
	}
}

func TestShellScript_ReadonlyAllowed(t *testing.T) {
	v := bashValidator(t, "readonly")
	req := shellReq(`[{"scriptType":"BASH","script":"readonly X=1"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when readonly is allowed: %v", err)
	}
}

func TestShellScript_HeredocAllowed(t *testing.T) {
	v := bashValidator(t, "cat")
	script := "cat <<EOF\nhello\nEOF"
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestShellScript_CommandWithPath(t *testing.T) {
	v := bashValidator(t, "/usr/bin/echo")
	req := shellReq(`[{"scriptType":"BASH","script":"/usr/bin/echo hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for full path command: %v", err)
	}
}

func TestShellScript_ScriptTypeCaseInsensitive(t *testing.T) {
	v := bashValidator(t, "echo")
	// scriptType in payload can be any case
	req := shellReq(`[{"scriptType":"Bash","script":"echo hello"}]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for mixed-case scriptType: %v", err)
	}
}

func TestShellScript_MultipleParameters(t *testing.T) {
	v := bashValidator(t, "echo", "curl")
	req := shellReq(`[
		{"scriptType":"BASH","script":"echo hello"},
		{"scriptType":"BASH","script":"curl http://example.com"}
	]`)
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for multiple params: %v", err)
	}
}

// --- Blocked scenarios ---

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

func TestShellScript_DynamicCommandNameBacktick(t *testing.T) {
	v := bashValidator(t, "echo", "get_cmd")
	script := "$(get_cmd) hello"
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for command substitution as command name")
	}
}

func TestShellScript_QuotedCommandNameDouble(t *testing.T) {
	v := bashValidator(t, "echo")
	// "echo" with quotes is not a Lit — rejected as dynamic
	script := `"echo" hello`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for double-quoted command name")
	}
}

func TestShellScript_QuotedCommandNameSingle(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `'echo' hello`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for single-quoted command name")
	}
}

func TestShellScript_ANSICQuotedCommandName(t *testing.T) {
	v := bashValidator(t, "ls")
	script := `$'\x6c\x73'`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for ANSI-C quoted command name")
	}
}

func TestShellScript_CommandSubstitutionInArgBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	// echo is allowed, but ls inside $() is not
	script := `echo $(ls)`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in substitution")
	}
}

func TestShellScript_BacktickSubstitutionBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	script := "echo `ls`"
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in backtick substitution")
	}
}

func TestShellScript_ProcessSubstitutionBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `echo <(ls)`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in process substitution")
	}
}

func TestShellScript_SubshellBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"(rm -rf /)"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in subshell")
	}
}

func TestShellScript_PipeSecondCommandBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"echo hello | grep foo"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in pipe")
	}
}

func TestShellScript_AndChainSecondBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"echo ok && rm file"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command after &&")
	}
}

func TestShellScript_OrChainSecondBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"echo ok || rm file"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command after ||")
	}
}

func TestShellScript_ExportNotAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"export FOO=bar"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error when export is not in allowlist")
	}
}

func TestShellScript_DeclareNotAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[{"scriptType":"BASH","script":"declare -i x=1"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error when declare is not in allowlist")
	}
}

func TestShellScript_LocalNotAllowed(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `local x=1`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error when local is not in allowlist")
	}
}

func TestShellScript_IfBodyBlocked(t *testing.T) {
	v := bashValidator(t, "true")
	script := `if true; then rm -rf /; fi`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in if body")
	}
}

func TestShellScript_ForBodyBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `for i in 1 2 3; do rm $i; done`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in for body")
	}
}

func TestShellScript_FunctionBodyBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	script := `myfunc() { rm -rf /; }`
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in function body")
	}
}

func TestShellScript_HeredocWithSubstitutionBlocked(t *testing.T) {
	v := bashValidator(t, "cat")
	script := "cat <<EOF\n$(rm -rf /)\nEOF"
	req := shellReq(`[{"scriptType":"BASH","script":` + jsonStr(script) + `}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for disallowed command in heredoc substitution")
	}
}

func TestShellScript_SecondParameterBlocked(t *testing.T) {
	v := bashValidator(t, "echo")
	req := shellReq(`[
		{"scriptType":"BASH","script":"echo ok"},
		{"scriptType":"BASH","script":"rm file"}
	]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error when second parameter has disallowed command")
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
	// unterminated if
	req := shellReq(`[{"scriptType":"BASH","script":"if true; then echo"}]`)
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for script with syntax error")
	}
}

// jsonStr JSON-encodes s for embedding in a JSON template.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
