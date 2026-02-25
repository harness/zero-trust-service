package zts

import (
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

type options struct {
	Port          int
	verifyHandler types.VerifyHandler
	metrics       *metrics.Metrics
	auditWriter   *audit.Writer
	auditHandler  *audit.Handler
}

func resolveOptions(opts ...Option) options {
	defaultOptions := options{
		Port:          8080,
		verifyHandler: DefaultVerifyHandler,
	}

	for _, opt := range opts {
		opt(&defaultOptions)
	}

	// Create default metrics only if none were injected via WithMetrics.
	if defaultOptions.metrics == nil {
		defaultOptions.metrics = metrics.New()
	}

	return defaultOptions
}

type Option func(*options)

func WithPort(port int) Option {
	if port <= 0 {
		panic("port must be greater than 0")
	}
	return func(o *options) {
		o.Port = port
	}
}

func WithVerifyHandler(handler types.VerifyHandler) Option {
	if handler == nil {
		panic("verify handler must not be nil")
	}
	return func(o *options) {
		o.verifyHandler = handler
	}
}

// WithMetrics allows injecting a custom Metrics instance.
// If not provided, a default instance is created.
func WithMetrics(m *metrics.Metrics) Option {
	if m == nil {
		panic("metrics must not be nil")
	}
	return func(o *options) {
		o.metrics = m
	}
}

// WithAuditWriter enables audit logging with the given writer.
func WithAuditWriter(w *audit.Writer) Option {
	if w == nil {
		panic("audit writer must not be nil")
	}
	return func(o *options) {
		o.auditWriter = w
	}
}

// WithAuditHandler registers the audit query API handler.
func WithAuditHandler(h *audit.Handler) Option {
	if h == nil {
		panic("audit handler must not be nil")
	}
	return func(o *options) {
		o.auditHandler = h
	}
}
