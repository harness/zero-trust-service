package verifier

import (
	"context"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// Instrumented wraps a verifier with metrics recording.
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

		if t := TrackerFrom(ctx); t != nil {
			t.Record(name, failed)
		}

		m.ValidatorEvaluationsTotal.Inc(name, result)
		m.ValidatorDuration.Observe(duration, name)

		if failed {
			accountID := request.ResolveAccountID()
			taskType := request.ResolveTaskType()
			m.BlockedTasksTotal.Inc(accountID, taskType, name)
		}

		return err
	})
}
