package metrics

// Counter is a monotonically increasing metric.
type Counter interface {
	Inc(labels ...string)
}

// Histogram records observed values (e.g. durations).
type Histogram interface {
	Observe(value float64, labels ...string)
}

// Gauge is a metric whose value can go up and down.
type Gauge interface {
	Set(value float64, labels ...string)
}

// Metrics holds all ZTS metric handles.
type Metrics struct {
	VerifyRequestsTotal   Counter
	VerifyRequestDuration Histogram

	ValidatorEvaluationsTotal Counter
	ValidatorDuration         Histogram

	BlockedTasksTotal    Counter
	MissingMetadataTotal Counter

	ValidatorsRegistered Gauge

	ResolverDuration Histogram
	ResolverTotal    Counter

	OutputRequestsTotal Counter
}

const (
	LabelStatusAuthorized   = "authorized"
	LabelStatusUnauthorized = "unauthorized"
	LabelStatusSuccess      = "success"
	LabelStatusError        = "error"
	LabelResultPass         = "pass"
	LabelResultFail         = "fail"

	LabelScopeGlobal   = "global"
	LabelScopeTaskType = "task_type"
	LabelScopeCustom   = "custom"

	LabelFieldZTSMetadata = "zts_metadata"
	LabelFieldAccountID   = "account_id"
	LabelFieldTaskType    = "task_type"

	LabelResolverSuccess = "success"
	LabelResolverError   = "error"
	LabelResolverSkipped = "skipped"
	LabelResolverInline  = "inline"
)
