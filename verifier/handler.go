package verifier

import (
	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"github.com/gotidy/ptr"
)

func ToHandler(verifier Interface) zts.VerifyHandler {
	return func(request zts.VerifyRequest) (zts.VerifyResponse, error) {
		if err := verifier.Handle(request); err != nil {
			return zts.VerifyResponse{
				Status: zts.VerifyStatusUnauthorized,
				Error:  ptr.Of(err.Error()),
			}, nil
		}
		return zts.VerifyResponse{
			Status: zts.VerifyStatusAuthorized,
		}, nil
	}
}
