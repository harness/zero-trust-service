package tasktype

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func passBuild(_ config.ValidatorDef) (verifier.Interface, error) {
	return verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil }), nil
}

func failBuild(_ config.ValidatorDef) (verifier.Interface, error) {
	return verifier.From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("blocked")
	}), nil
}

func TestDispatcher_MatchingTaskType(t *testing.T) {
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "test_v"}},
	}

	d, _, err := NewDispatcher(byTaskType, failBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "test_v"}},
	}

	d, _, err := NewDispatcher(byTaskType, failBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "test_v"}},
	}

	d, _, err := NewDispatcher(byTaskType, passBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for nil task details, got %v", err)
	}
}

func TestDispatcher_EmptyTaskType(t *testing.T) {
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "test_v"}},
	}

	d, _, err := NewDispatcher(byTaskType, passBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: ""},
		},
	}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for empty task type, got %v", err)
	}
}

func TestDispatcher_DisabledValidator(t *testing.T) {
	disabled := false
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "test_v", Enabled: &disabled}},
	}

	d, _, err := NewDispatcher(byTaskType, failBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Disabled validator should not run, so no chain for this task type
	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "SHELL_SCRIPT"},
		},
	}
	if err := d.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass when all validators disabled, got %v", err)
	}
}

func TestDispatcher_BuildError(t *testing.T) {
	byTaskType := map[string][]config.ValidatorDef{
		"SHELL_SCRIPT": {{Type: "bad"}},
	}

	errBuild := func(_ config.ValidatorDef) (verifier.Interface, error) {
		return nil, errors.New("build failed")
	}

	_, _, err := NewDispatcher(byTaskType, errBuild)
	if err == nil {
		t.Fatal("expected error from build failure")
	}
}
