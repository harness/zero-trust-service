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
	"log"
	"time"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// Logging logs each verify request: a "processing" line on entry and an
// outcome line ("authorized" / "denied" / "internal error") on exit, with
// task_id, account_id, and duration.
func Logging() zts.VerifyMiddleware {
	return func(next types.VerifyHandler) types.VerifyHandler {
		return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
			start := time.Now()
			taskID, accountID := req.TaskID(), req.ResolveAccountID()
			log.Printf("[verify] processing request task_id=%s account_id=%s task_type=%s",
				taskID, accountID, req.ResolveTaskType())

			resp, err := next(ctx, req)
			duration := time.Since(start)

			switch {
			case err != nil:
				log.Printf("[verify] internal error task_id=%s account_id=%s duration=%s error=%v",
					taskID, accountID, duration, err)
			case !resp.Allowed:
				log.Printf("[verify] denied task_id=%s account_id=%s duration=%s reason=%s",
					taskID, accountID, duration, resp.Reason)
			default:
				log.Printf("[verify] authorized task_id=%s account_id=%s duration=%s",
					taskID, accountID, duration)
			}
			return resp, err
		}
	}
}
