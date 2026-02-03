package zts

type options struct {
	Port          int
	verifyHandler VerifyHandler
}

func resolveOptions(opts ...Option) options {
	defaultOptions := options{
		Port:          8080,
		verifyHandler: DefaultVerifyHandler,
	}

	for _, opt := range opts {
		opt(&defaultOptions)
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

func WithVerifyHandler(handler VerifyHandler) Option {
	if handler == nil {
		panic("verify handler must not be nil")
	}
	return func(o *options) {
		o.verifyHandler = handler
	}
}
