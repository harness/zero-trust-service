package verifier

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestFrom_Pass(t *testing.T) {
	v := From(func(_ context.Context, _ types.VerifyRequest) error {
		return nil
	})
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFrom_Fail(t *testing.T) {
	want := errors.New("boom")
	v := From(func(_ context.Context, _ types.VerifyRequest) error {
		return want
	})
	err := v.Handle(context.Background(), types.VerifyRequest{})
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestFrom_ReceivesRequest(t *testing.T) {
	v := From(func(_ context.Context, req types.VerifyRequest) error {
		if req.TaskPackage == nil || req.TaskPackage.TaskID != "abc" {
			t.Errorf("expected task id abc, got %v", req.TaskPackage)
		}
		return nil
	})
	_ = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.TaskPackage{TaskID: "abc"},
	})
}
