package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metric name constants — single place to audit all metric names.
const (
	VerifyRequestsTotalName   = "zts_verify_requests_total"
	VerifyRequestDurationName = "zts_verify_request_duration_seconds"

	ValidatorEvaluationsTotalName = "zts_validator_evaluations_total"
	ValidatorDurationName         = "zts_validator_duration_seconds"

	BlockedTasksTotalName    = "zts_blocked_tasks_total"
	MissingMetadataTotalName = "zts_missing_metadata_total"

	ValidatorsRegisteredName = "zts_validators_registered"
)

// Label value constants — avoids typos and makes labels grep-able.
const (
	// Status labels
	LabelStatusAuthorized   = "authorized"
	LabelStatusUnauthorized = "unauthorized"
	LabelStatusError        = "error"
	LabelResultPass         = "pass"
	LabelResultFail         = "fail"

	// Scope labels
	LabelScopeGlobal   = "global"
	LabelScopeTaskType = "task_type"
	LabelScopeCustom   = "custom"

	// Missing metadata field labels
	LabelFieldZTSMetadata = "zts_metadata"
	LabelFieldAccountID   = "account_id"
	LabelFieldTaskType    = "task_type"
)

// Metrics holds all ZTS prometheus metrics, grouped by concern.
type Metrics struct {
	// Request-level
	VerifyRequestsTotal   *prometheus.CounterVec
	VerifyRequestDuration *prometheus.HistogramVec

	// Validator-level
	ValidatorEvaluationsTotal *prometheus.CounterVec
	ValidatorDuration         *prometheus.HistogramVec

	// Security / audit
	BlockedTasksTotal    *prometheus.CounterVec
	MissingMetadataTotal *prometheus.CounterVec

	// Operational
	ValidatorsRegistered *prometheus.GaugeVec
}

// New creates and registers all ZTS prometheus metrics.
func New() *Metrics {
	return &Metrics{
		VerifyRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: VerifyRequestsTotalName,
			Help: "Total number of verification requests",
		}, []string{"status", "account_id"}),

		VerifyRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    VerifyRequestDurationName,
			Help:    "Duration of verification requests in seconds",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}, []string{"status"}),

		ValidatorEvaluationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: ValidatorEvaluationsTotalName,
			Help: "Total validator evaluations",
		}, []string{"validator", "result"}),

		ValidatorDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    ValidatorDurationName,
			Help:    "Duration of individual validator evaluations in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1},
		}, []string{"validator"}),

		BlockedTasksTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: BlockedTasksTotalName,
			Help: "Total tasks blocked by validators",
		}, []string{"account_id", "task_type", "validator"}),

		MissingMetadataTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MissingMetadataTotalName,
			Help: "Requests arriving with missing critical metadata fields",
		}, []string{"field"}),

		ValidatorsRegistered: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: ValidatorsRegisteredName,
			Help: "Number of active validators by scope",
		}, []string{"scope"}),
	}
}
