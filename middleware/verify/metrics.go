// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package verify

import (
	"context"
	"time"

	zts "github.com/harness/zero-trust-service"
	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/types"
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
