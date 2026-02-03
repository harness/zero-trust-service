package verifier

import (
	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
)

func Chain(verifiers ...Interface) Interface {
	return From(func(request zts.VerifyRequest) error {
		for _, verifier := range verifiers {
			if err := verifier.Handle(request); err != nil {
				return err
			}
		}
		return nil
	})
}
