package verifier

import (
	"context"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func Chain(verifiers ...Interface) Interface {
	return From(func(ctx context.Context, request types.VerifyRequest) error {
		for _, verifier := range verifiers {
			if err := verifier.Handle(ctx, request); err != nil {
				return err
			}
		}
		return nil
	})
}
