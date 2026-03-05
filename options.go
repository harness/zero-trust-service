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
	auditWriter   audit.Writer
}

func resolveOptions(opts ...Option) options {
	o := options{
		Port:          8080,
		verifyHandler: DefaultVerifyHandler,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.metrics == nil {
		o.metrics = metrics.NewNoop()
	}
	return o
}

type Option func(*options)

func WithPort(port int) Option {
	if port <= 0 {
		panic("port must be greater than 0")
	}
	return func(o *options) { o.Port = port }
}

func WithVerifyHandler(handler types.VerifyHandler) Option {
	if handler == nil {
		panic("verify handler must not be nil")
	}
	return func(o *options) { o.verifyHandler = handler }
}

func WithMetrics(m *metrics.Metrics) Option {
	if m == nil {
		panic("metrics must not be nil")
	}
	return func(o *options) { o.metrics = m }
}

func WithAuditWriter(w audit.Writer) Option {
	if w == nil {
		panic("audit writer must not be nil")
	}
	return func(o *options) { o.auditWriter = w }
}
