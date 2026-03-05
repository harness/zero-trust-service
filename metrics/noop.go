package metrics

type noopCounter struct{}

func (noopCounter) Inc(...string) {}

type noopHistogram struct{}

func (noopHistogram) Observe(float64, ...string) {}

type noopGauge struct{}

func (noopGauge) Set(float64, ...string) {}

// NewNoop returns a Metrics where every operation is a silent no-op.
func NewNoop() *Metrics {
	return &Metrics{
		VerifyRequestsTotal:   noopCounter{},
		VerifyRequestDuration: noopHistogram{},

		ValidatorEvaluationsTotal: noopCounter{},
		ValidatorDuration:         noopHistogram{},

		BlockedTasksTotal:    noopCounter{},
		MissingMetadataTotal: noopCounter{},

		ValidatorsRegistered: noopGauge{},

		ResolverDuration: noopHistogram{},
		ResolverTotal:    noopCounter{},

		OutputRequestsTotal: noopCounter{},
	}
}
