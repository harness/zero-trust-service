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
