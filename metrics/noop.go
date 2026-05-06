package metrics

type noop struct{}

func (noop) Counter(string, float64, ...Dimension)   {}
func (noop) Histogram(string, float64, ...Dimension)  {}
func (noop) Gauge(string, float64, ...Dimension)      {}

// NewNoop returns an Emitter where every operation is a silent no-op.
func NewNoop() Emitter { return noop{} }
