package instrumented

import (
	"context"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
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
