package metrics

import "testing"

func TestNewNoop_AllFieldsPopulated(t *testing.T) {
	m := NewNoop()

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

func TestNewNoop_OperationsDoNotPanic(t *testing.T) {
	m := NewNoop()
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

func TestLabelConstants(t *testing.T) {
	labels := []string{
		LabelStatusAuthorized, LabelStatusUnauthorized,
		LabelStatusSuccess, LabelStatusError,
		LabelResultPass, LabelResultFail,
		LabelScopeGlobal, LabelScopeTaskType, LabelScopeCustom,
		LabelFieldZTSMetadata, LabelFieldAccountID, LabelFieldTaskType,
		LabelResolverSuccess, LabelResolverError, LabelResolverSkipped, LabelResolverInline,
	}
	for _, l := range labels {
		if l == "" {
			t.Error("found empty label constant")
		}
	}
}
