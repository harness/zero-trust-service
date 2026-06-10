package verify

import (
	"context"
	"encoding/json"
	"time"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/requestctx"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/instrumented"
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
