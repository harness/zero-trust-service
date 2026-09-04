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

package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/harness/zero-trust-service/audit"
	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/requestctx"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier/instrumented"
)

// ---- helpers ----------------------------------------------------------------

func allowHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{Allowed: true}, nil
}

func denyHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{Allowed: false, Reason: "policy"}, nil
}

func errHandler(_ context.Context, _ types.VerifyRequest) (types.VerifyResponse, error) {
	return types.VerifyResponse{}, errors.New("internal failure")
}

func reqWithMeta(accountID, taskType string) types.VerifyRequest {
	return types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: accountID},
			TaskDetails: &types.TaskDetails{TaskType: taskType},
		},
	}
}

// captureWriter records WriteEvent calls.
type captureWriter struct {
	kind    string
	record  audit.AuditRecord
	payload json.RawMessage
}

func (c *captureWriter) WriteEvent(kind string, rec audit.AuditRecord, raw json.RawMessage) {
	c.kind = kind
	c.record = rec
	c.payload = raw
}

// ---- Logging ----------------------------------------------------------------

func TestLogging(t *testing.T) {
	tests := []struct {
		name    string
		handler types.VerifyHandler
		account string
		want    string
	}{
		{"authorized", allowHandler, "acc1", "authorized"},
		{"denied", denyHandler, "acc2", "denied"},
		{"error", errHandler, "acc3", "internal error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			orig := log.Writer()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(orig) })

			mw := Logging()
			_, _ = mw(tc.handler)(context.Background(), reqWithMeta(tc.account, "SHELL_SCRIPT"))

			out := buf.String()
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected %q in log, got: %s", tc.want, out)
			}
			if tc.name == "authorized" && !strings.Contains(out, tc.account) {
				t.Errorf("expected account_id in log, got: %s", out)
			}
		})
	}
}

// ---- Metrics ----------------------------------------------------------------

func TestMetrics_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil emitter")
		}
	}()
	Metrics(nil)
}

func TestMetrics(t *testing.T) {
	tests := []struct {
		name        string
		handler     types.VerifyHandler
		wantErr     bool
		wantAllowed bool
	}{
		{"authorized", allowHandler, false, true},
		{"denied", denyHandler, false, false},
		{"error", errHandler, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mw := Metrics(metrics.NewNoop())
			resp, err := mw(tc.handler)(context.Background(), reqWithMeta("acc1", "SHELL_SCRIPT"))
			if tc.wantErr && err == nil {
				t.Fatal("expected error to propagate")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr && resp.Allowed != tc.wantAllowed {
				t.Errorf("expected allowed=%v, got %v", tc.wantAllowed, resp.Allowed)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		resp types.VerifyResponse
		err  error
		want string
	}{
		{types.VerifyResponse{Allowed: true}, nil, statusAuthorized},
		{types.VerifyResponse{Allowed: false}, nil, statusUnauthorized},
		{types.VerifyResponse{}, errors.New("boom"), statusError},
	}
	for _, tt := range tests {
		got := classify(tt.resp, tt.err)
		if got != tt.want {
			t.Errorf("classify(%v, %v) = %q, want %q", tt.resp, tt.err, got, tt.want)
		}
	}
}

// ---- Audit ------------------------------------------------------------------

func TestAudit_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil writer")
		}
	}()
	Audit(nil)
}

func TestAudit_WritesRecord_Allowed(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	ctx := requestctx.WithRawPayload(context.Background(), []byte(`{"taskId":"t1"}`))
	ctx = instrumented.WithTracker(ctx, instrumented.NewTracker())

	resp, err := mw(allowHandler)(ctx, reqWithMeta("acc1", "SHELL_SCRIPT"))
	if err != nil || !resp.Allowed {
		t.Fatalf("unexpected resp=%v err=%v", resp, err)
	}
	if w.kind != audit.EventVerify {
		t.Errorf("expected kind=%q, got %q", audit.EventVerify, w.kind)
	}
	rec, ok := w.record.(audit.Record)
	if !ok {
		t.Fatalf("expected audit.Record, got %T", w.record)
	}
	if !rec.Allowed {
		t.Error("expected record.Allowed=true")
	}
	if rec.AccountID != "acc1" {
		t.Errorf("expected accountID=acc1, got %q", rec.AccountID)
	}
	if string(w.payload) != `{"taskId":"t1"}` {
		t.Errorf("unexpected payload: %s", w.payload)
	}
}

func TestAudit_WritesRecord_Denied(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	_, _ = mw(denyHandler)(context.Background(), reqWithMeta("acc1", "SHELL_SCRIPT"))

	rec, ok := w.record.(audit.Record)
	if !ok {
		t.Fatalf("expected audit.Record, got %T", w.record)
	}
	if rec.Allowed {
		t.Error("expected record.Allowed=false")
	}
	if rec.Reason != "policy" {
		t.Errorf("expected reason=policy, got %q", rec.Reason)
	}
}

func TestAudit_WritesRecord_Error(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	_, _ = mw(errHandler)(context.Background(), reqWithMeta("acc1", "SHELL_SCRIPT"))

	rec, ok := w.record.(audit.Record)
	if !ok {
		t.Fatalf("expected audit.Record, got %T", w.record)
	}
	if rec.Error == "" {
		t.Error("expected non-empty record.Error")
	}
}

// ---- MissingMetadata --------------------------------------------------------

func TestMissingMetadata_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil emitter")
		}
	}()
	MissingMetadata(nil)
}

func TestMissingMetadata_PassThrough_Complete(t *testing.T) {
	m := metrics.NewNoop()
	mw := MissingMetadata(m)
	resp, err := mw(allowHandler)(context.Background(), reqWithMeta("acc1", "SHELL_SCRIPT"))
	if err != nil || !resp.Allowed {
		t.Fatalf("expected allowed, got resp=%v err=%v", resp, err)
	}
}

func TestMissingMetadata_PassThrough_Nil(t *testing.T) {
	m := metrics.NewNoop()
	mw := MissingMetadata(m)
	// nil TaskPackage — should still call next and not panic
	resp, err := mw(allowHandler)(context.Background(), types.VerifyRequest{})
	if err != nil || !resp.Allowed {
		t.Fatalf("expected allowed, got resp=%v err=%v", resp, err)
	}
}
