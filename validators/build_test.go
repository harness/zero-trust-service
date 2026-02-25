package validators

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"github.com/prometheus/client_golang/prometheus"
)

func testMetrics() *metrics.Metrics {
	reg := prometheus.NewRegistry()
	m := &metrics.Metrics{
		VerifyRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "t_verify_requests_total",
		}, []string{"status", "account_id"}),
		VerifyRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "t_verify_request_duration_seconds",
		}, []string{"status"}),
		ValidatorEvaluationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "t_validator_evaluations_total",
		}, []string{"validator", "result"}),
		ValidatorDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "t_validator_duration_seconds",
		}, []string{"validator"}),
		BlockedTasksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "t_blocked_tasks_total",
		}, []string{"account_id", "task_type", "validator"}),
		MissingMetadataTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "t_missing_metadata_total",
		}, []string{"field"}),
		ValidatorsRegistered: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "t_validators_registered",
		}, []string{"scope"}),
	}
	reg.MustRegister(
		m.ValidatorEvaluationsTotal,
		m.ValidatorDuration,
		m.BlockedTasksTotal,
		m.ValidatorsRegistered,
	)
	return m
}

func TestBuildFromConfig_GlobalOnly(t *testing.T) {
	m := testMetrics()
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
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

	// Should pass for acc1
	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	// Should fail for acc2
	req2 := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc2"}}}
	if err := chain.Handle(context.Background(), req2); err == nil {
		t.Fatal("expected error for blocked account")
	}
}

func TestBuildFromConfig_WithTaskType(t *testing.T) {
	m := testMetrics()
	cfg := config.ValidatorsConfig{
		ByTaskType: map[string][]config.ValidatorDef{
			"SHELL_SCRIPT_TASK_NG": {
				{
					Type: "command_blocklist",
					Config: map[string]any{
						"patterns": []any{"rm -rf /"},
					},
				},
			},
		},
	}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-matching task type should pass
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
	m := testMetrics()
	cfg := config.ValidatorsConfig{}

	chain, err := BuildFromConfig(cfg, m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty config should produce a no-op chain
	if err := chain.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for empty config, got %v", err)
	}
}

func TestBuildFromConfig_DisabledValidator(t *testing.T) {
	m := testMetrics()
	disabled := false
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
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

	// Disabled validator should not run — any request should pass
	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "unknown"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass with disabled validator, got %v", err)
	}
}

func TestBuildFromConfig_UnknownValidatorType(t *testing.T) {
	m := testMetrics()
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
			{Type: "nonexistent"},
		},
	}

	_, err := BuildFromConfig(cfg, m)
	if err == nil {
		t.Fatal("expected error for unknown validator type")
	}
}
