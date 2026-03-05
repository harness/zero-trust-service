package prometheus

import (
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
)

func TestNewWithRegistry_AllFieldsPopulated(t *testing.T) {
	m := NewWithRegistry(prom.NewRegistry())

	if m.VerifyRequestsTotal == nil {
		t.Error("VerifyRequestsTotal is nil")
	}
	if m.VerifyRequestDuration == nil {
		t.Error("VerifyRequestDuration is nil")
	}
	if m.ValidatorEvaluationsTotal == nil {
		t.Error("ValidatorEvaluationsTotal is nil")
	}
	if m.ValidatorDuration == nil {
		t.Error("ValidatorDuration is nil")
	}
	if m.BlockedTasksTotal == nil {
		t.Error("BlockedTasksTotal is nil")
	}
	if m.MissingMetadataTotal == nil {
		t.Error("MissingMetadataTotal is nil")
	}
	if m.ValidatorsRegistered == nil {
		t.Error("ValidatorsRegistered is nil")
	}
	if m.ResolverDuration == nil {
		t.Error("ResolverDuration is nil")
	}
	if m.ResolverTotal == nil {
		t.Error("ResolverTotal is nil")
	}
	if m.OutputRequestsTotal == nil {
		t.Error("OutputRequestsTotal is nil")
	}
}

func TestNewWithRegistry_OperationsDoNotPanic(t *testing.T) {
	m := NewWithRegistry(prom.NewRegistry())
	m.VerifyRequestsTotal.Inc("status", "account")
	m.VerifyRequestDuration.Observe(0.5, "status")
	m.ValidatorEvaluationsTotal.Inc("v", "pass")
	m.ValidatorDuration.Observe(0.1, "v")
	m.BlockedTasksTotal.Inc("acc", "type", "v")
	m.MissingMetadataTotal.Inc("field")
	m.ValidatorsRegistered.Set(3, "global")
	m.ResolverDuration.Observe(1.0, "success")
	m.ResolverTotal.Inc("success")
	m.OutputRequestsTotal.Inc("success", "acc")
}

func TestNewWithRegistry_MultipleInstances(t *testing.T) {
	m1 := NewWithRegistry(prom.NewRegistry())
	m2 := NewWithRegistry(prom.NewRegistry())

	m1.VerifyRequestsTotal.Inc("s", "a")
	m2.VerifyRequestsTotal.Inc("s", "b")
}
