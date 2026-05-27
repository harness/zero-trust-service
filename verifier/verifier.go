package verifier

import (
	"context"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

type Interface interface {
	Handle(ctx context.Context, request types.VerifyRequest) error
}

type middlewareFunc func(ctx context.Context, request types.VerifyRequest) error

func (f middlewareFunc) Handle(ctx context.Context, request types.VerifyRequest) error {
	return f(ctx, request)
}

func From(f func(ctx context.Context, request types.VerifyRequest) error) Interface {
	return middlewareFunc(f)
}

// Wrap applies middlewares to a verifier in outermost-first order:
// the first middleware in the slice sees the request before the rest.
//
//	v := verifier.Wrap(handler, mw1, mw2, mw3)
//	// request flow: mw1 → mw2 → mw3 → handler
func Wrap(v Interface, mws ...func(next Interface) Interface) Interface {
	for i := len(mws) - 1; i >= 0; i-- {
		v = mws[i](v)
	}
	return v
}
