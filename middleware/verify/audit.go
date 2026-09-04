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
	"encoding/json"
	"time"

	zts "github.com/harness/zero-trust-service"
	"github.com/harness/zero-trust-service/audit"
	"github.com/harness/zero-trust-service/requestctx"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier/instrumented"
	"github.com/google/uuid"
)

// Audit writes an audit.Record for each verify request via the supplied
// audit.Writer. The raw request body is read from context (set by the
// HTTP handler via requestctx.WithRawPayload) so the audit captures the
// exact bytes received from the client. The writer decides whether to
// persist synchronously or asynchronously.
func Audit(w audit.Writer) zts.VerifyMiddleware {
	if w == nil {
		panic("verify.Audit: audit writer must not be nil")
	}
	return func(next types.VerifyHandler) types.VerifyHandler {
		return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			end := time.Now()

			record := buildRecord(start, end, req, ctx, resp, err)
			rawPayload := json.RawMessage(requestctx.RawPayloadFrom(ctx))
			w.WriteEvent(audit.EventVerify, record, rawPayload)

			return resp, err
		}
	}
}

func buildRecord(
	start, end time.Time,
	req types.VerifyRequest,
	ctx context.Context,
	resp types.VerifyResponse,
	err error,
) audit.Record {
	var validatorsRun []string
	failedValidator := ""
	if t := instrumented.TrackerFrom(ctx); t != nil {
		validatorsRun, failedValidator = t.Results()
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	return audit.Record{
		ID:                 uuid.New().String(),
		StartTime:          start.UTC(),
		EndTime:            end.UTC(),
		AccountID:          req.ResolveAccountID(),
		TaskID:             req.TaskID(),
		TaskType:           req.ResolveTaskType(),
		DelegateID:         req.DelegateID(),
		DelegateInstanceID: req.DelegateInstanceID(),
		GitOpsAgentID:      req.GitOpsAgentID(),
		Allowed:            err == nil && resp.Allowed,
		Reason:             resp.Reason,
		Error:              errStr,
		FailedValidator:    failedValidator,
		Duration:           end.Sub(start),
		ValidatorsRun:      validatorsRun,
	}
}
