package metrics

// Dimension is a key-value pair attached to a metric observation.
type Dimension struct {
	Key   string
	Value string
}

// Dim is a shorthand constructor for a Dimension.
func Dim(key, value string) Dimension {
	return Dimension{Key: key, Value: value}
}

// Emitter is the interface for recording metrics. Packages define their own
// metric names and dimensions; the metrics package only provides this interface.
type Emitter interface {
	Counter(name string, value float64, dims ...Dimension)
	Histogram(name string, value float64, dims ...Dimension)
	Gauge(name string, value float64, dims ...Dimension)
}
