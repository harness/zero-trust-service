package verifier

import (
	"context"
	"errors"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestToHandler_Authorized(t *testing.T) {
	v := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	handler := ToHandler(v)

	resp, err := handler(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	if resp.Reason != "" {
		t.Fatalf("expected empty reason, got %q", resp.Reason)
	}
}

func TestToHandler_Unauthorized(t *testing.T) {
	v := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("not allowed")
	})
	handler := ToHandler(v)

	resp, err := handler(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Fatalf("expected allowed=false, got true")
	}
	if resp.Reason != "not allowed" {
		t.Fatalf("expected reason 'not allowed', got %q", resp.Reason)
	}
}
