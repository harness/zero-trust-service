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

	zts "github.com/harness/zero-trust-service"
	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/types"
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
	if req.TaskPackage.ZTSMetadata == nil && req.TaskPackage.GitOpsAgentID == "" {
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
