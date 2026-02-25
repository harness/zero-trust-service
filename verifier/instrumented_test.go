package verifier

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"github.com/prometheus/client_golang/prometheus"
)

// newTestMetrics creates a Metrics instance with a custom registry to avoid
// global registration conflicts across tests.
func newTestMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()
	reg := prometheus.NewRegistry()

	m := &metrics.Metrics{
		VerifyRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_verify_requests_total",
		}, []string{"status", "account_id"}),
		VerifyRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "test_verify_request_duration_seconds",
		}, []string{"status"}),
		ValidatorEvaluationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_validator_evaluations_total",
		}, []string{"validator", "result"}),
		ValidatorDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "test_validator_duration_seconds",
		}, []string{"validator"}),
		BlockedTasksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_blocked_tasks_total",
		}, []string{"account_id", "task_type", "validator"}),
		MissingMetadataTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_missing_metadata_total",
		}, []string{"field"}),
		ValidatorsRegistered: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "test_validators_registered",
		}, []string{"scope"}),
	}

	reg.MustRegister(
		m.ValidatorEvaluationsTotal,
		m.ValidatorDuration,
		m.BlockedTasksTotal,
	)

	return m
}

func TestInstrumented_Pass(t *testing.T) {
	m := newTestMetrics(t)
	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Instrumented("test_v", inner, m)

	err := v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestInstrumented_Fail_RecordsBlocked(t *testing.T) {
	m := newTestMetrics(t)
	inner := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("denied")
	})
	v := Instrumented("test_v", inner, m)

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			AccountID: "acc1",
			TaskDetails: &types.TaskDetails{
				TaskType: "SHELL_SCRIPT",
			},
		},
	}
	err := v.Handle(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstrumented_RecordsInTracker(t *testing.T) {
	m := newTestMetrics(t)
	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Instrumented("my_validator", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	run, failed := tracker.Results()
	if len(run) != 1 || run[0] != "my_validator" {
		t.Fatalf("expected [my_validator], got %v", run)
	}
	if failed != "" {
		t.Fatalf("expected no failure, got %q", failed)
	}
}

func TestInstrumented_RecordsFailureInTracker(t *testing.T) {
	m := newTestMetrics(t)
	inner := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("nope")
	})
	v := Instrumented("bad_v", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	_, failed := tracker.Results()
	if failed != "bad_v" {
		t.Fatalf("expected bad_v, got %q", failed)
	}
}
