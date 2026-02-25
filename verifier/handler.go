package verifier

import (
	"context"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func ToHandler(v Interface) types.VerifyHandler {
	return func(ctx context.Context, request types.VerifyRequest) (types.VerifyResponse, error) {
		if err := v.Handle(ctx, request); err != nil {
			return types.VerifyResponse{
				Allowed: false,
				Reason:  err.Error(),
			}, nil
		}
		return types.VerifyResponse{Allowed: true}, nil
	}
}
