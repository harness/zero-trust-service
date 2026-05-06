package verifier

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

var passValidator = From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
var failValidator = From(func(_ context.Context, _ types.VerifyRequest) error { return errors.New("blocked") })

func TestDispatcher_MatchingTaskType(t *testing.T) {
	d := NewDispatcher(map[string]Interface{
		"SHELL_SCRIPT": failValidator,
	})

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "SHELL_SCRIPT"},
		},
	}
	if err := d.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for matching task type")
	}
}

func TestDispatcher_NonMatchingTaskType(t *testing.T) {
	d := NewDispatcher(map[string]Interface{
		"SHELL_SCRIPT": failValidator,
	})

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "OTHER_TASK"},
		},
	}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for non-matching task type, got %v", err)
	}
}

func TestDispatcher_NilTaskDetails(t *testing.T) {
	d := NewDispatcher(map[string]Interface{
		"SHELL_SCRIPT": passValidator,
	})

	req := types.VerifyRequest{}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil task details, got %v", err)
	}
}

func TestDispatcher_EmptyTaskType(t *testing.T) {
	d := NewDispatcher(map[string]Interface{
		"SHELL_SCRIPT": passValidator,
	})

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: ""},
		},
	}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for empty task type, got %v", err)
	}
}
