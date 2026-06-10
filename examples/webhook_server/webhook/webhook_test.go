package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestWebhook_Authorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL, Name: "test-hook"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.TaskPackage{TaskID: "t1"},
	})
	if err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestWebhook_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": false, "reason": "policy denied"})
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL, Name: "test-hook"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.TaskPackage{TaskID: "t1"},
	})
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestWebhook_FailOpen(t *testing.T) {
	v, err := New(Config{
		URL:      "http://127.0.0.1:1",
		FailOpen: true,
		Timeout:  "100ms",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected pass with fail_open, got %v", err)
	}
}

func TestWebhook_FailClosed(t *testing.T) {
	v, err := New(Config{
		URL:     "http://127.0.0.1:1",
		Timeout: "100ms",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil {
		t.Fatal("expected error for unreachable with fail_open=false")
	}
}

func TestWebhook_ConfigErrors(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestWebhook_InvalidTimeout(t *testing.T) {
	_, err := New(Config{URL: "http://example.com", Timeout: "not-a-duration"})
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}
