package zts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestHandleVerify_Authorized(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(
		WithMetrics(m),
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{Allowed: true}, nil
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
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
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Allowed {
		t.Errorf("expected allowed=true, got false")
	}
}

func TestHandleVerify_Unauthorized(t *testing.T) {
	m := testMetricsForRoot()
	reason := "blocked by policy"
	s := NewServer(
		WithMetrics(m),
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{
				Allowed: false,
				Reason:  reason,
			}, nil
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{TaskID: "t1"},
	})
	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp types.VerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Allowed {
		t.Errorf("expected allowed=false, got true")
	}
	if resp.Reason != reason {
		t.Errorf("expected reason %q, got %q", reason, resp.Reason)
	}
}

func TestHandleVerify_HandlerError(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(
		WithMetrics(m),
		WithVerifyHandler(func(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
			return types.VerifyResponse{}, errors.New("internal failure")
		}),
	)

	body, _ := json.Marshal(types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{TaskID: "t1"},
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
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m))

	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleVerify_EmptyBody(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m))

	req := httptest.NewRequest("POST", "/api/verify", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(w, req)

	// Default handler authorizes everything
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
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

func TestRecordMissingMetadata_NilTaskPackage(t *testing.T) {
	m := testMetricsForRoot()
	req := types.VerifyRequest{}
	recordMissingMetadata(req, m)
}

func TestRecordMissingMetadata_NilZTSMetadata(t *testing.T) {
	m := testMetricsForRoot()
	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{},
	}
	recordMissingMetadata(req, m)
}

func TestRecordMissingMetadata_MissingAccountID(t *testing.T) {
	m := testMetricsForRoot()
	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{},
		},
	}
	recordMissingMetadata(req, m)
}

func TestRecordMissingMetadata_MissingTaskType(t *testing.T) {
	m := testMetricsForRoot()
	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"},
		},
	}
	recordMissingMetadata(req, m)
}
