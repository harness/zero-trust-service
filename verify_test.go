package zts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/requestctx"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

// errReader always returns an error from Read, used to simulate io.ReadAll failure.
type errReader struct{ err error }

func (r errReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestHandleVerify_Authorized(t *testing.T) {
	s := NewServer(
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{Allowed: true}, nil
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			TaskID:    "t1",
			AccountID: "acc1",
		},
	})

	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp types.VerifyResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Allowed {
		t.Errorf("expected allowed=true, got false")
	}
}

func TestHandleVerify_Unauthorized(t *testing.T) {
	reason := "blocked by policy"
	s := NewServer(
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{Allowed: false, Reason: reason}, nil
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.TaskPackage{TaskID: "t1"},
	})
	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp types.VerifyResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Allowed {
		t.Errorf("expected allowed=false, got true")
	}
	if resp.Reason != reason {
		t.Errorf("expected reason %q, got %q", reason, resp.Reason)
	}
}

func TestHandleVerify_HandlerError(t *testing.T) {
	s := NewServer(
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{}, errors.New("internal failure")
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.TaskPackage{TaskID: "t1"},
	})
	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleVerify_InvalidJSON(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleVerify_EmptyBody(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleVerify_RawPayloadInContext(t *testing.T) {
	var got []byte
	s := NewServer(
		WithVerifyHandler(func(ctx context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			got = requestctx.RawPayloadFrom(ctx)
			return types.VerifyResponse{Allowed: true}, nil
		}),
	)

	body := []byte(`{"taskPackage":{"taskId":"t1"}}`)
	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if !bytes.Equal(got, body) {
		t.Fatalf("expected raw payload %q in context, got %q", body, got)
	}
}

func TestHandleOutput_OK(t *testing.T) {
	s := NewServer(
		WithOutputHandler(func(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
			return types.OutputResponse{}, nil
		}),
	)

	body, _ := json.Marshal(types.OutputRequest{TaskID: "t1"})
	req := httptest.NewRequest("POST", "/api/output", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleOutput_InvalidJSON(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest("POST", "/api/output", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleOutput_HandlerError(t *testing.T) {
	s := NewServer(
		WithOutputHandler(func(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
			return types.OutputResponse{}, errors.New("output failure")
		}),
	)

	body, _ := json.Marshal(types.OutputRequest{TaskID: "t1"})
	req := httptest.NewRequest("POST", "/api/output", bytes.NewReader(body))
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleOutput_RawPayloadInContext(t *testing.T) {
	var got []byte
	s := NewServer(
		WithOutputHandler(func(ctx context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
			got = requestctx.RawPayloadFrom(ctx)
			return types.OutputResponse{}, nil
		}),
	)

	body := []byte(`{"taskId":"t1"}`)
	req := httptest.NewRequest("POST", "/api/output", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if !bytes.Equal(got, body) {
		t.Fatalf("expected raw payload %q in context, got %q", body, got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate([]byte("hello"), 10); got != "hello" {
		t.Errorf("truncate short = %q, want hello", got)
	}
	if got := truncate([]byte("hello world"), 5); got != "hello..." {
		t.Errorf("truncate long = %q, want hello...", got)
	}
}

func TestDefaultVerifyHandler(t *testing.T) {
	resp, err := DefaultVerifyHandler(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Errorf("expected allowed=true, got false")
	}
}

func TestDefaultOutputHandler(t *testing.T) {
	_, err := DefaultOutputHandler(context.Background(), types.OutputRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleVerify_BodyReadError(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest("POST", "/api/verify", errReader{errors.New("disk error")})
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on body read error, got %d", w.Code)
	}
}

func TestHandleOutput_BodyReadError(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest("POST", "/api/output", errReader{errors.New("disk error")})
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on body read error, got %d", w.Code)
	}
}
