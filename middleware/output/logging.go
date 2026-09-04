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
	"log"

	zts "github.com/harness/zero-trust-service"
	"github.com/harness/zero-trust-service/types"
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
