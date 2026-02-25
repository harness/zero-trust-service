package zts

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"github.com/prometheus/client_golang/prometheus"
)

// testMetricsForRoot creates a Metrics instance using a custom registry
// to avoid duplicate registration panics across tests.
func testMetricsForRoot() *metrics.Metrics {
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
	reg.MustRegister(m.VerifyRequestsTotal, m.VerifyRequestDuration)
	return m
}

func TestResolveOptions_Defaults(t *testing.T) {
	m := testMetricsForRoot()
	opts := resolveOptions(WithMetrics(m))

	if opts.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", opts.Port)
	}
	if opts.verifyHandler == nil {
		t.Fatal("expected default verify handler")
	}
	if opts.metrics == nil {
		t.Fatal("expected metrics")
	}
	if opts.auditWriter != nil {
		t.Error("expected nil audit writer by default")
	}
	if opts.auditHandler != nil {
		t.Error("expected nil audit handler by default")
	}
}

func TestWithPort(t *testing.T) {
	m := testMetricsForRoot()
	opts := resolveOptions(WithMetrics(m), WithPort(9090))
	if opts.Port != 9090 {
		t.Errorf("expected port 9090, got %d", opts.Port)
	}
}

func TestWithPort_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for port <= 0")
		}
	}()
	WithPort(0)
}

func TestWithVerifyHandler(t *testing.T) {
	called := false
	handler := func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
		called = true
		return types.VerifyResponse{Allowed: true}, nil
	}

	m := testMetricsForRoot()
	opts := resolveOptions(WithMetrics(m), WithVerifyHandler(handler))
	opts.verifyHandler(context.Background(), types.VerifyRequest{})

	if !called {
		t.Fatal("custom handler was not called")
	}
}

func TestWithVerifyHandler_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil handler")
		}
	}()
	WithVerifyHandler(nil)
}

func TestWithMetrics_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil metrics")
		}
	}()
	WithMetrics(nil)
}

func TestWithAuditWriter_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil audit writer")
		}
	}()
	WithAuditWriter(nil)
}

func TestWithAuditHandler_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil audit handler")
		}
	}()
	WithAuditHandler(nil)
}
