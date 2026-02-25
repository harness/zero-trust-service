package tasktype

import (
	"context"
	"encoding/json"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestCommandBlocklist_Blocked(t *testing.T) {
	v, err := CommandBlocklist(map[string]any{
		"patterns": []any{"rm -rf /", "mkfs."},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "SHELL_SCRIPT_TASK_NG",
				Parameters: json.RawMessage(`[{"command":"rm -rf / --no-preserve-root"}]`),
			},
		},
	}
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected blocked")
	}
}

func TestCommandBlocklist_Allowed(t *testing.T) {
	v, err := CommandBlocklist(map[string]any{
		"patterns": []any{"rm -rf /"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "SHELL_SCRIPT_TASK_NG",
				Parameters: json.RawMessage(`[{"command":"echo hello"}]`),
			},
		},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestCommandBlocklist_NilTaskDetails(t *testing.T) {
	v, err := CommandBlocklist(map[string]any{
		"patterns": []any{"rm -rf /"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil task details, got %v", err)
	}
}

func TestCommandBlocklist_EmptyParameters(t *testing.T) {
	v, err := CommandBlocklist(map[string]any{
		"patterns": []any{"rm -rf /"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "SHELL_SCRIPT_TASK_NG"},
		},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for empty params, got %v", err)
	}
}

func TestCommandBlocklist_ConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
	}{
		{"missing key", map[string]any{}},
		{"not a list", map[string]any{"patterns": "not-a-list"}},
		{"empty list", map[string]any{"patterns": []any{}}},
		{"non-string item", map[string]any{"patterns": []any{123}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CommandBlocklist(tc.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
