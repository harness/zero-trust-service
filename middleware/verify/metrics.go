package verify

import (
	"context"
	"time"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

const (
	metricVerifyRequestsTotal   = "zts_verify_requests_total"
	metricVerifyRequestDuration = "zts_verify_request_duration_seconds"
)

// Metrics emits counter (zts_verify_requests_total) and histogram
// (zts_verify_request_duration_seconds) for each verify request,
// dimensioned by status (authorized/unauthorized/error) and account_id.
func Metrics(m metrics.Emitter) zts.VerifyMiddleware {
	if m == nil {
		panic("verify.Metrics: metrics emitter must not be nil")
	}
	return func(next types.VerifyHandler) types.VerifyHandler {
		return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start)

			status := classify(resp, err)
			accountID := req.ResolveAccountID()

			m.Counter(metricVerifyRequestsTotal, 1,
				metrics.Dim(keyStatus, status), metrics.Dim(keyAccountID, accountID))
			m.Histogram(metricVerifyRequestDuration, duration.Seconds(),
				metrics.Dim(keyStatus, status), metrics.Dim(keyAccountID, accountID))

			return resp, err
		}
	}
}

func classify(resp types.VerifyResponse, err error) string {
	switch {
	case err != nil:
		return statusError
	case resp.Allowed:
		return statusAuthorized
	default:
		return statusUnauthorized
	}
}
