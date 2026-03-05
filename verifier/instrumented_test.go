package verifier

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestInstrumented_Pass(t *testing.T) {
	m := metrics.NewNoop()
	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Instrumented("test_v", inner, m)

	err := v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestInstrumented_Fail_RecordsBlocked(t *testing.T) {
	m := metrics.NewNoop()
	inner := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("denied")
	})
	v := Instrumented("test_v", inner, m)

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			AccountID: "acc1",
			TaskDetails: &types.TaskDetails{
				TaskType: "SHELL_SCRIPT",
			},
		},
	}
	err := v.Handle(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstrumented_RecordsInTracker(t *testing.T) {
	m := metrics.NewNoop()
	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Instrumented("my_validator", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	run, failed := tracker.Results()
	if len(run) != 1 || run[0] != "my_validator" {
		t.Fatalf("expected [my_validator], got %v", run)
	}
	if failed != "" {
		t.Fatalf("expected no failure, got %q", failed)
	}
}

func TestInstrumented_RecordsFailureInTracker(t *testing.T) {
	m := metrics.NewNoop()
	inner := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("nope")
	})
	v := Instrumented("bad_v", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	_, failed := tracker.Results()
	if failed != "bad_v" {
		t.Fatalf("expected bad_v, got %q", failed)
	}
}
