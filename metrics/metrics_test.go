package metrics

import "testing"

func TestNew_AllMetricsInitialized(t *testing.T) {
	m := New()

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
}

func TestMetricConstants(t *testing.T) {
	// Ensure metric names follow the zts_ prefix convention
	names := []string{
		VerifyRequestsTotalName,
		VerifyRequestDurationName,
		ValidatorEvaluationsTotalName,
		ValidatorDurationName,
		BlockedTasksTotalName,
		MissingMetadataTotalName,
		ValidatorsRegisteredName,
	}

	for _, name := range names {
		if len(name) < 4 || name[:4] != "zts_" {
			t.Errorf("metric name %q should start with 'zts_'", name)
		}
	}
}

func TestLabelConstants(t *testing.T) {
	// Ensure label constants are non-empty
	labels := []string{
		LabelStatusError,
		LabelResultPass,
		LabelResultFail,
		LabelScopeGlobal,
		LabelScopeTaskType,
		LabelScopeCustom,
		LabelFieldZTSMetadata,
		LabelFieldAccountID,
		LabelFieldTaskType,
	}

	for _, label := range labels {
		if label == "" {
			t.Error("found empty label constant")
		}
	}
}
