package verifier

import (
	"context"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// Instrumented wraps a verifier with prometheus metrics:
//   - zts_validator_evaluations_total
//   - zts_validator_duration_seconds
//   - zts_blocked_tasks_total
func Instrumented(name string, v Interface, m *metrics.Metrics) Interface {
	return From(func(ctx context.Context, request types.VerifyRequest) error {
		start := time.Now()
		err := v.Handle(ctx, request)
		duration := time.Since(start).Seconds()

		failed := err != nil
		result := metrics.LabelResultPass
		if failed {
			result = metrics.LabelResultFail
		}

		// Record in tracker for audit (tracker lives in context, not on the request)
		if t := TrackerFrom(ctx); t != nil {
			t.Record(name, failed)
		}

		m.ValidatorEvaluationsTotal.WithLabelValues(name, result).Inc()
		m.ValidatorDuration.WithLabelValues(name).Observe(duration)

		if failed {
			accountID := request.ResolveAccountID()
			taskType := request.ResolveTaskType()
			m.BlockedTasksTotal.WithLabelValues(accountID, taskType, name).Inc()
		}

		return err
	})
}
