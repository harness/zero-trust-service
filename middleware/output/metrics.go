package output

import (
	"context"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

const metricOutputRequestsTotal = "zts_output_requests_total"

// Metrics emits zts_output_requests_total dimensioned by status
// (success/error) and account_id.
func Metrics(m metrics.Emitter) zts.OutputMiddleware {
	if m == nil {
		panic("output.Metrics: metrics emitter must not be nil")
	}
	return func(next types.OutputHandler) types.OutputHandler {
		return func(ctx context.Context, req types.OutputRequest) (types.OutputResponse, error) {
			resp, err := next(ctx, req)

			status := statusSuccess
			if err != nil {
				status = statusError
			}
			m.Counter(metricOutputRequestsTotal, 1,
				metrics.Dim(keyStatus, status), metrics.Dim(keyAccountID, req.AccountID()))

			return resp, err
		}
	}
}
