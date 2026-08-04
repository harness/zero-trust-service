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

func TestWrap_NoMiddlewares(t *testing.T) {
	v := Wrap(From(func(_ context.Context, _ types.VerifyRequest) error { return nil }))
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWrap_OutermostFirst(t *testing.T) {
	var order []string
	mw := func(name string) func(Interface) Interface {
		return func(next Interface) Interface {
			return From(func(ctx context.Context, req types.VerifyRequest) error {
				order = append(order, name+":before")
				err := next.Handle(ctx, req)
				order = append(order, name+":after")
				return err
			})
		}
	}

	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Wrap(inner, mw("a"), mw("b"), mw("c"))
	_ = v.Handle(context.Background(), types.VerifyRequest{})

	want := []string{"a:before", "b:before", "c:before", "c:after", "b:after", "a:after"}
	if len(order) != len(want) {
		t.Fatalf("expected %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("step %d: want %q got %q", i, want[i], order[i])
		}
	}
}

func TestWrap_ErrorPropagates(t *testing.T) {
	inner := From(func(_ context.Context, _ types.VerifyRequest) error { return errors.New("boom") })
	v := Wrap(inner)
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err == nil || err.Error() != "boom" {
		t.Fatalf("expected 'boom', got %v", err)
	}
}

func TestWrap_MiddlewareShortCircuits(t *testing.T) {
	called := false
	inner := From(func(_ context.Context, _ types.VerifyRequest) error {
		called = true
		return nil
	})
	blockMW := func(_ Interface) Interface {
		return From(func(_ context.Context, _ types.VerifyRequest) error {
			return errors.New("blocked")
		})
	}
	v := Wrap(inner, blockMW)
	err := v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil || err.Error() != "blocked" {
		t.Fatalf("expected 'blocked', got %v", err)
	}
	if called {
		t.Error("inner verifier should not have been called")
	}
}
