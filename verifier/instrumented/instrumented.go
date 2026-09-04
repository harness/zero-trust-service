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

package instrumented

import (
	"context"
	"time"

	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
)

const (
	metricVerifierEvaluationsTotal = "zts_verifier_evaluations_total"
	metricVerifierDuration         = "zts_verifier_duration_seconds"
	metricBlockedTasksTotal        = "zts_blocked_tasks_total"

	resultPass = "pass"
	resultFail = "fail"

	keyValidator = "validator"
	keyResult    = "result"
	keyAccountID = "account_id"
	keyTaskType  = "task_type"
)

// Wrap wraps a verifier with metrics recording and tracker updates.
func Wrap(name string, v verifier.Interface, m metrics.Emitter) verifier.Interface {
	return verifier.From(func(ctx context.Context, request types.VerifyRequest) error {
		start := time.Now()
		err := v.Handle(ctx, request)
		duration := time.Since(start).Seconds()

		failed := err != nil
		result := resultPass
		if failed {
			result = resultFail
		}

		accountID := request.ResolveAccountID()

		if t := TrackerFrom(ctx); t != nil {
			t.Record(name, failed)
		}

		m.Counter(metricVerifierEvaluationsTotal, 1, metrics.Dim(keyValidator, name), metrics.Dim(keyResult, result), metrics.Dim(keyAccountID, accountID))
		m.Histogram(metricVerifierDuration, duration, metrics.Dim(keyValidator, name), metrics.Dim(keyAccountID, accountID))

		if failed {
			taskType := request.ResolveTaskType()
			m.Counter(metricBlockedTasksTotal, 1,
				metrics.Dim(keyAccountID, accountID),
				metrics.Dim(keyTaskType, taskType),
				metrics.Dim(keyValidator, name),
			)
		}

		return err
	})
}
