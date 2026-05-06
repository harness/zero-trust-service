package prometheus

import (
	"sync"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Option configures the prometheus Emitter.
type Option func(*emitter)

// WithBuckets sets custom histogram buckets for a specific metric name.
func WithBuckets(name string, buckets []float64) Option {
	return func(e *emitter) { e.buckets[name] = buckets }
}

type emitter struct {
	mu         sync.RWMutex
	factory    promauto.Factory
	counters   map[string]*prom.CounterVec
	histograms map[string]*prom.HistogramVec
	gauges     map[string]*prom.GaugeVec
	buckets    map[string][]float64
}

// New creates an Emitter backed by Prometheus using the default registry.
func New(opts ...Option) metrics.Emitter {
	return NewWithRegistry(nil, opts...)
}

// NewWithRegistry creates an Emitter using a custom Prometheus registry.
// Pass nil to use the default global registry.
func NewWithRegistry(reg prom.Registerer, opts ...Option) metrics.Emitter {
	if reg == nil {
		reg = prom.DefaultRegisterer
	}
	e := &emitter{
		factory:    promauto.With(reg),
		counters:   make(map[string]*prom.CounterVec),
		histograms: make(map[string]*prom.HistogramVec),
		gauges:     make(map[string]*prom.GaugeVec),
		buckets:    make(map[string][]float64),
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

func (e *emitter) Counter(name string, value float64, dims ...metrics.Dimension) {
	keys, vals := split(dims)
	cv := e.getOrCreateCounter(name, keys)
	cv.WithLabelValues(vals...).Add(value)
}

func (e *emitter) Histogram(name string, value float64, dims ...metrics.Dimension) {
	keys, vals := split(dims)
	hv := e.getOrCreateHistogram(name, keys)
	hv.WithLabelValues(vals...).Observe(value)
}

func (e *emitter) Gauge(name string, value float64, dims ...metrics.Dimension) {
	keys, vals := split(dims)
	gv := e.getOrCreateGauge(name, keys)
	gv.WithLabelValues(vals...).Set(value)
}

func (e *emitter) getOrCreateCounter(name string, keys []string) *prom.CounterVec {
	e.mu.RLock()
	cv, ok := e.counters[name]
	e.mu.RUnlock()
	if ok {
		return cv
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if cv, ok = e.counters[name]; ok {
		return cv
	}
	cv = e.factory.NewCounterVec(prom.CounterOpts{Name: name}, keys)
	e.counters[name] = cv
	return cv
}

func (e *emitter) getOrCreateHistogram(name string, keys []string) *prom.HistogramVec {
	e.mu.RLock()
	hv, ok := e.histograms[name]
	e.mu.RUnlock()
	if ok {
		return hv
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if hv, ok = e.histograms[name]; ok {
		return hv
	}
	buckets := e.buckets[name]
	if buckets == nil {
		buckets = prom.DefBuckets
	}
	hv = e.factory.NewHistogramVec(prom.HistogramOpts{Name: name, Buckets: buckets}, keys)
	e.histograms[name] = hv
	return hv
}

func (e *emitter) getOrCreateGauge(name string, keys []string) *prom.GaugeVec {
	e.mu.RLock()
	gv, ok := e.gauges[name]
	e.mu.RUnlock()
	if ok {
		return gv
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if gv, ok = e.gauges[name]; ok {
		return gv
	}
	gv = e.factory.NewGaugeVec(prom.GaugeOpts{Name: name}, keys)
	e.gauges[name] = gv
	return gv
}

// split converts dims into parallel key/value slices. Duplicate keys are
// collapsed with last-wins semantics to avoid Prometheus panicking on
// repeated label names.
func split(dims []metrics.Dimension) (keys, vals []string) {
	if len(dims) == 0 {
		return nil, nil
	}
	keys = make([]string, 0, len(dims))
	vals = make([]string, 0, len(dims))
	idx := make(map[string]int, len(dims))
	for _, d := range dims {
		if i, ok := idx[d.Key]; ok {
			vals[i] = d.Value
			continue
		}
		idx[d.Key] = len(keys)
		keys = append(keys, d.Key)
		vals = append(vals, d.Value)
	}
	return
}
