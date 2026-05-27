package output

import (
	"context"
	"log"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// Logging logs each output request with account_id, task_id, task_type,
// and response_code.
func Logging() zts.OutputMiddleware {
	return func(next types.OutputHandler) types.OutputHandler {
		return func(ctx context.Context, req types.OutputRequest) (types.OutputResponse, error) {
			log.Printf("[output] received task output account_id=%s task_id=%s task_type=%s response_code=%s",
				req.AccountID(), req.TaskID, req.TaskTypeName(), req.ResponseCode())
			return next(ctx, req)
		}
	}
}
