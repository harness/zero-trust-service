// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": true})
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
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": false, "reason": "policy denied"})
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

func TestWebhook_Non2xx_FailOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL, FailOpen: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass with fail_open on non-2xx, got %v", err)
	}
}

func TestWebhook_Non2xx_FailClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err == nil {
		t.Fatal("expected error for non-2xx with fail_closed")
	}
}

func TestWebhook_InvalidResponseJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err == nil {
		t.Fatal("expected error for invalid response JSON")
	}
}

func TestWebhook_AllowedStatusCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer srv.Close()

	v, err := New(Config{URL: srv.URL, AllowedStatusCodes: []int{202}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for 202 in allowed status codes, got %v", err)
	}
}
