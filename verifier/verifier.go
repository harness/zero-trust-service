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
