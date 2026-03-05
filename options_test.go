package zts

import (
	"context"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestResolveOptions_Defaults(t *testing.T) {
	opts := resolveOptions()
	if opts.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", opts.Port)
	}
	if opts.verifyHandler == nil {
		t.Fatal("expected default verify handler")
	}
	if opts.metrics == nil {
		t.Fatal("expected noop metrics when none provided")
	}
	if opts.auditWriter != nil {
		t.Error("expected nil audit writer by default")
	}
	if opts.auditRoutes != nil {
		t.Error("expected nil audit routes by default")
	}
	if opts.adminRoutes != nil {
		t.Error("expected nil admin routes by default")
	}
}

func TestWithPort(t *testing.T) {
	opts := resolveOptions(WithMetrics(metrics.NewNoop()), WithPort(9090))
	if opts.Port != 9090 {
		t.Errorf("expected port 9090, got %d", opts.Port)
	}
}

func TestWithPort_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for port <= 0")
		}
	}()
	WithPort(0)
}

func TestWithVerifyHandler(t *testing.T) {
	called := false
	handler := func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
		called = true
		return types.VerifyResponse{Allowed: true}, nil
	}

	opts := resolveOptions(WithMetrics(metrics.NewNoop()), WithVerifyHandler(handler))
	opts.verifyHandler(context.Background(), types.VerifyRequest{})

	if !called {
		t.Fatal("custom handler was not called")
	}
}

func TestWithVerifyHandler_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil handler")
		}
	}()
	WithVerifyHandler(nil)
}

func TestWithMetrics_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil metrics")
		}
	}()
	WithMetrics(nil)
}

func TestWithAuditWriter_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil audit writer")
		}
	}()
	WithAuditWriter(nil)
}

func TestWithAuditRoutes_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil audit route registrar")
		}
	}()
	WithAuditRoutes(nil)
}

func TestWithAdminRoutes_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil admin route registrar")
		}
	}()
	WithAdminRoutes(nil)
}
