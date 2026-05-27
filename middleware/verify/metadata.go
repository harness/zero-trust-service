package verify

import (
	"context"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

const metricMissingMetadataTotal = "zts_missing_metadata_total"

// MissingMetadata increments zts_missing_metadata_total when the incoming
// request is missing zts_metadata, account_id, or task_type. Exists so
// operators can spot delegates / clients sending malformed requests
// without changing the verify outcome.
func MissingMetadata(m metrics.Emitter) zts.VerifyMiddleware {
	if m == nil {
		panic("verify.MissingMetadata: metrics emitter must not be nil")
	}
	return func(next types.VerifyHandler) types.VerifyHandler {
		return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
			recordMissingMetadata(req, m)
			return next(ctx, req)
		}
	}
}

func recordMissingMetadata(req types.VerifyRequest, m metrics.Emitter) {
	if req.TaskPackage == nil {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldZTSMetadata))
		return
	}
	if req.TaskPackage.ZTSMetadata == nil {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldZTSMetadata))
		return
	}
	if req.ResolveAccountID() == "" {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldAccountID))
	}
	if req.ResolveTaskType() == "" {
		m.Counter(metricMissingMetadataTotal, 1, metrics.Dim(keyField, fieldTaskType))
	}
}
