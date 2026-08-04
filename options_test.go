package zts

import (
	"context"
	"errors"
	"testing"

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
	if opts.outputHandler == nil {
		t.Fatal("expected default output handler")
	}
	if len(opts.verifyMiddleware) != 0 {
		t.Errorf("expected no verify middleware by default, got %d", len(opts.verifyMiddleware))
	}
	if len(opts.outputMiddleware) != 0 {
		t.Errorf("expected no output middleware by default, got %d", len(opts.outputMiddleware))
	}
}

func TestWithPort(t *testing.T) {
	opts := resolveOptions(WithPort(9090))
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

	opts := resolveOptions(WithVerifyHandler(handler))
	_, _ = opts.verifyHandler(context.Background(), types.VerifyRequest{})

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

func TestWithOutputHandler(t *testing.T) {
	called := false
	handler := func(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
		called = true
		return types.OutputResponse{}, nil
	}
	opts := resolveOptions(WithOutputHandler(handler))
	_, _ = opts.outputHandler(context.Background(), types.OutputRequest{})
	if !called {
		t.Fatal("custom output handler was not called")
	}
}

func TestWithOutputHandler_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil output handler")
		}
	}()
	WithOutputHandler(nil)
}

func TestWithVerifyMiddleware_OutermostFirst(t *testing.T) {
	var calls []string

	mw := func(name string) VerifyMiddleware {
		return func(next types.VerifyHandler) types.VerifyHandler {
			return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
				calls = append(calls, name+":pre")
				resp, err := next(ctx, req)
				calls = append(calls, name+":post")
				return resp, err
			}
		}
	}

	opts := resolveOptions(
		WithVerifyMiddleware(mw("a"), mw("b")),
		WithVerifyMiddleware(mw("c")),
	)
	h := opts.composedVerifyHandler()
	_, _ = h(context.Background(), types.VerifyRequest{})

	want := []string{"a:pre", "b:pre", "c:pre", "c:post", "b:post", "a:post"}
	if len(calls) != len(want) {
		t.Fatalf("expected %d calls, got %d: %v", len(want), len(calls), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d: expected %q, got %q", i, want[i], calls[i])
		}
	}
}

func TestWithOutputMiddleware_OutermostFirst(t *testing.T) {
	var calls []string

	mw := func(name string) OutputMiddleware {
		return func(next types.OutputHandler) types.OutputHandler {
			return func(ctx context.Context, req types.OutputRequest) (types.OutputResponse, error) {
				calls = append(calls, name+":pre")
				resp, err := next(ctx, req)
				calls = append(calls, name+":post")
				return resp, err
			}
		}
	}

	opts := resolveOptions(WithOutputMiddleware(mw("a"), mw("b")))
	h := opts.composedOutputHandler()
	_, _ = h(context.Background(), types.OutputRequest{})

	want := []string{"a:pre", "b:pre", "b:post", "a:post"}
	if len(calls) != len(want) {
		t.Fatalf("expected %d calls, got %d: %v", len(want), len(calls), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d: expected %q, got %q", i, want[i], calls[i])
		}
	}
}

func TestWithVerifyMiddleware_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	mw := func(next types.VerifyHandler) types.VerifyHandler {
		return func(ctx context.Context, req types.VerifyRequest) (types.VerifyResponse, error) {
			return next(ctx, req)
		}
	}
	handler := func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
		return types.VerifyResponse{}, wantErr
	}
	opts := resolveOptions(WithVerifyHandler(handler), WithVerifyMiddleware(mw))
	_, err := opts.composedVerifyHandler()(context.Background(), types.VerifyRequest{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to propagate, got %v", err)
	}
}

func TestWithVerifyMiddleware_NilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil verify middleware")
		}
	}()
	WithVerifyMiddleware(nil)
}

func TestWithOutputMiddleware_NilPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil output middleware")
		}
	}()
	WithOutputMiddleware(nil)
}

