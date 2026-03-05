package prometheus

import (
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"github.com/go-chi/chi/v5"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler serves /metrics on a chi.Router.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Handle("/metrics", promhttp.Handler())
}

const (
	VerifyRequestsTotalName          = "zts_verify_requests_total"
	VerifyRequestDurationSecondsName = "zts_verify_request_duration_seconds"

	ValidatorEvaluationsTotalName    = "zts_validator_evaluations_total"
	ValidatorDurationSecondsName     = "zts_validator_duration_seconds"

	BlockedTasksTotalName    = "zts_blocked_tasks_total"
	MissingMetadataTotalName = "zts_missing_metadata_total"

	ValidatorsRegisteredName = "zts_validators_registered"

	ResolverDurationSecondsName = "zts_resolver_duration_seconds"
	ResolverTotalName           = "zts_resolver_total"

	OutputRequestsTotalName = "zts_output_requests_total"

	// Label name constants for Prometheus label keys.
	labelStatus    = "status"
	labelAccountID = "account_id"
	labelValidator = "validator"
	labelResult    = "result"
	labelTaskType  = "task_type"
	labelField     = "field"
	labelScope     = "scope"
)

// New creates a Metrics backed by Prometheus using the default registry.
func New() *metrics.Metrics {
	return NewWithRegistry(nil)
}

// NewWithRegistry creates a Metrics using a custom Prometheus registry.
// Pass nil to use the default global registry.
func NewWithRegistry(reg prom.Registerer) *metrics.Metrics {
	if reg == nil {
		reg = prom.DefaultRegisterer
	}
	factory := promauto.With(reg)

	return &metrics.Metrics{
		VerifyRequestsTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: VerifyRequestsTotalName,
			Help: "Total number of verification requests",
		}, []string{labelStatus, labelAccountID})},

		VerifyRequestDuration: &histogramVec{factory.NewHistogramVec(prom.HistogramOpts{
			Name:    VerifyRequestDurationSecondsName,
			Help:    "Duration of verification requests in seconds",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}, []string{labelStatus})},

		ValidatorEvaluationsTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: ValidatorEvaluationsTotalName,
			Help: "Total validator evaluations",
		}, []string{labelValidator, labelResult})},

		ValidatorDuration: &histogramVec{factory.NewHistogramVec(prom.HistogramOpts{
			Name:    ValidatorDurationSecondsName,
			Help:    "Duration of individual validator evaluations in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1},
		}, []string{labelValidator})},

		BlockedTasksTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: BlockedTasksTotalName,
			Help: "Total tasks blocked by validators",
		}, []string{labelAccountID, labelTaskType, labelValidator})},

		MissingMetadataTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: MissingMetadataTotalName,
			Help: "Requests arriving with missing critical metadata fields",
		}, []string{labelField})},

		ValidatorsRegistered: &gaugeVec{factory.NewGaugeVec(prom.GaugeOpts{
			Name: ValidatorsRegisteredName,
			Help: "Number of active validators by scope",
		}, []string{labelScope})},

		ResolverDuration: &histogramVec{factory.NewHistogramVec(prom.HistogramOpts{
			Name:    ResolverDurationSecondsName,
			Help:    "Duration of pipeline YAML resolution in seconds",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{labelStatus})},

		ResolverTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: ResolverTotalName,
			Help: "Total pipeline YAML resolution attempts",
		}, []string{labelStatus})},

		OutputRequestsTotal: &counterVec{factory.NewCounterVec(prom.CounterOpts{
			Name: OutputRequestsTotalName,
			Help: "Total task output requests received from delegates",
		}, []string{labelStatus, labelAccountID})},
	}
}

type counterVec struct{ v *prom.CounterVec }

func (c *counterVec) Inc(labels ...string) { c.v.WithLabelValues(labels...).Inc() }

type histogramVec struct{ v *prom.HistogramVec }

func (h *histogramVec) Observe(value float64, labels ...string) {
	h.v.WithLabelValues(labels...).Observe(value)
}

type gaugeVec struct{ v *prom.GaugeVec }

func (g *gaugeVec) Set(value float64, labels ...string) {
	g.v.WithLabelValues(labels...).Set(value)
}
