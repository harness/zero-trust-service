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
	"encoding/json"
	"time"

	zts "github.com/harness/zero-trust-service"
	"github.com/harness/zero-trust-service/audit"
	"github.com/harness/zero-trust-service/requestctx"
	"github.com/harness/zero-trust-service/types"
	"github.com/google/uuid"
)

// Audit writes an audit.OutputRecord for each output request via the
// supplied audit.Writer. The raw request body is read from context (set
// by the HTTP handler via requestctx.WithRawPayload). The writer decides
// whether to persist synchronously or asynchronously.
func Audit(w audit.Writer) zts.OutputMiddleware {
	if w == nil {
		panic("output.Audit: audit writer must not be nil")
	}
	return func(next types.OutputHandler) types.OutputHandler {
		return func(ctx context.Context, req types.OutputRequest) (types.OutputResponse, error) {
			resp, err := next(ctx, req)

			record := audit.OutputRecord{
				ID:           uuid.New().String(),
				Timestamp:    time.Now().UTC().UnixMilli(),
				AccountID:    req.AccountID(),
				TaskID:       req.TaskID,
				TaskTypeName: req.TaskTypeName(),
				ResponseCode: req.ResponseCode(),
				GitOpsAgentID: req.GitOpsAgentID(),
			}
			rawPayload := json.RawMessage(requestctx.RawPayloadFrom(ctx))
			w.WriteEvent(audit.EventOutput, record, rawPayload)

			return resp, err
		}
	}
}
