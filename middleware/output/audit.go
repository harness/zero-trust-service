package output

import (
	"context"
	"encoding/json"
	"time"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/requestctx"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
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
			}
			rawPayload := json.RawMessage(requestctx.RawPayloadFrom(ctx))
			w.WriteEvent(audit.EventOutput, record, rawPayload)

			return resp, err
		}
	}
}
