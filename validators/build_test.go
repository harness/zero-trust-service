package validators

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestBuildFromConfig_GlobalOnly(t *testing.T) {
	m := metrics.NewNoop()
	cfg := ValidatorsConfig{
		Global: []ValidatorDef{
			{
				Type: "require_account",
				Config: map[string]any{
					"allowed_accounts": []any{"acc1"},
				},
			},
		},
	}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	req2 := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc2"}}}
	if err := chain.Handle(context.Background(), req2); err == nil {
		t.Fatal("expected error for blocked account")
	}
}

func TestBuildFromConfig_WithTaskType(t *testing.T) {
	m := metrics.NewNoop()
	cfg := ValidatorsConfig{
		ByTaskType: map[string][]ValidatorDef{
			"SHELL_SCRIPT_TASK_NG": {
				{
					Type: "shellscript",
					Config: map[string]any{
						"bash": []any{"rm"},
					},
				},
			},
		},
	}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "OTHER"},
		},
	}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for non-matching task type, got %v", err)
	}
}

func TestBuildFromConfig_Empty(t *testing.T) {
	m := metrics.NewNoop()
	cfg := ValidatorsConfig{}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := chain.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for empty config, got %v", err)
	}
}

func TestBuildFromConfig_DisabledValidator(t *testing.T) {
	m := metrics.NewNoop()
	disabled := false
	cfg := ValidatorsConfig{
		Global: []ValidatorDef{
			{
				Type:    "require_account",
				Enabled: &disabled,
				Config: map[string]any{
					"allowed_accounts": []any{"acc1"},
				},
			},
		},
	}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "unknown"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass with disabled validator, got %v", err)
	}
}

func TestBuildFromConfig_UnknownValidatorType(t *testing.T) {
	m := metrics.NewNoop()
	cfg := ValidatorsConfig{
		Global: []ValidatorDef{
			{Type: "nonexistent"},
		},
	}

	_, err := BuildFromConfig(cfg, m)
	if err == nil {
		t.Fatal("expected error for unknown validator type")
	}
}
