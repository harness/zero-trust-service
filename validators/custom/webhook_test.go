package custom

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

	v, err := Webhook(map[string]any{
		"url":  srv.URL,
		"name": "test-hook",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{TaskID: "t1"},
	})
	if err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestWebhook_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"allowed": false,
			"reason":  "policy denied",
		})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{
		"url":  srv.URL,
		"name": "test-hook",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{TaskID: "t1"},
	})
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestWebhook_UnauthorizedNoReasonMsg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": false})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWebhook_HTTPError_FailClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{
		"url":       srv.URL,
		"fail_open": false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil {
		t.Fatal("expected error for 500 with fail_open=false")
	}
}

func TestWebhook_HTTPError_FailOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{
		"url":       srv.URL,
		"fail_open": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected pass with fail_open, got %v", err)
	}
}

func TestWebhook_Unreachable_FailOpen(t *testing.T) {
	v, err := Webhook(map[string]any{
		"url":       "http://127.0.0.1:1", // unreachable
		"fail_open": true,
		"timeout":   "100ms",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected pass with fail_open, got %v", err)
	}
}

func TestWebhook_Unreachable_FailClosed(t *testing.T) {
	v, err := Webhook(map[string]any{
		"url":     "http://127.0.0.1:1",
		"timeout": "100ms",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil {
		t.Fatal("expected error for unreachable with fail_open=false")
	}
}

func TestWebhook_AllowedStatusCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	// Without allowed_status_codes, 201 is OK (in 200-299)
	v, err := Webhook(map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for 201 in default range, got %v", err)
	}

	// With explicit allowed_status_codes=[200], 201 should fail
	v2, err := Webhook(map[string]any{
		"url":                  srv.URL,
		"allowed_status_codes": []any{200},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v2.Handle(context.Background(), types.VerifyRequest{}); err == nil {
		t.Fatal("expected error for 201 not in allowed_status_codes=[200]")
	}
}

func TestWebhook_CustomHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{
		"url": srv.URL,
		"headers": map[string]any{
			"Authorization": "Bearer secret-token",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = v.Handle(context.Background(), types.VerifyRequest{})
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
}

func TestWebhook_CustomMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{
		"url":    srv.URL,
		"method": "PUT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = v.Handle(context.Background(), types.VerifyRequest{})
	if gotMethod != "PUT" {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
}

func TestWebhook_DefaultMethodIsPOST(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = v.Handle(context.Background(), types.VerifyRequest{})
	if gotMethod != "POST" {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
}

func TestWebhook_SendsRequestBody(t *testing.T) {
	var gotTaskID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req types.VerifyRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TaskPackage != nil {
			gotTaskID = req.TaskPackage.TaskID
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = v.Handle(context.Background(), types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{TaskID: "task-123"},
	})
	if gotTaskID != "task-123" {
		t.Fatalf("expected task-123, got %q", gotTaskID)
	}
}

func TestWebhook_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	v, err := Webhook(map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = v.Handle(context.Background(), types.VerifyRequest{})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestWebhook_ConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
	}{
		{"missing url", map[string]any{}},
		{"empty url", map[string]any{"url": ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Webhook(tc.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestWebhook_InvalidTimeout(t *testing.T) {
	_, err := Webhook(map[string]any{
		"url":     "http://example.com",
		"timeout": "not-a-duration",
	})
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
		ok    bool
	}{
		{200, 200, true},
		{float64(201), 201, true},
		{"nope", 0, false},
		{nil, 0, false},
	}

	for _, tc := range tests {
		got, ok := toInt(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Errorf("toInt(%v) = (%d, %v), want (%d, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}
