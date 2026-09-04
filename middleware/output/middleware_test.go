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

package output

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
)

// ---- helpers ----------------------------------------------------------------

func okHandler(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
	return types.OutputResponse{}, nil
}

func errHandler(_ context.Context, _ types.OutputRequest) (types.OutputResponse, error) {
	return types.OutputResponse{Error: "boom"}, errors.New("boom")
}

func req(accountID, taskID, taskType string) types.OutputRequest {
	return types.OutputRequest{
		TaskID: taskID,
		TaskResponse: &types.TaskOutputResponse{
			AccountID:    accountID,
			TaskTypeName: taskType,
			ResponseCode: "OK",
		},
	}
}

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

func TestLogging_LogsFields(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	mw := Logging()
	_, _ = mw(okHandler)(context.Background(), req("acc1", "task-1", "SHELL_SCRIPT"))

	out := buf.String()
	for _, want := range []string{"acc1", "task-1", "SHELL_SCRIPT"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in log output, got: %s", want, out)
		}
	}
}

func TestLogging_PassesThrough(t *testing.T) {
	mw := Logging()
	_, err := mw(errHandler)(context.Background(), req("acc1", "t1", "SHELL_SCRIPT"))
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected 'boom' error to propagate, got %v", err)
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

func TestMetrics_Success(t *testing.T) {
	m := metrics.NewNoop()
	mw := Metrics(m)
	_, err := mw(okHandler)(context.Background(), req("acc1", "t1", "SHELL_SCRIPT"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetrics_Error(t *testing.T) {
	m := metrics.NewNoop()
	mw := Metrics(m)
	_, err := mw(errHandler)(context.Background(), req("acc1", "t1", "SHELL_SCRIPT"))
	if err == nil {
		t.Fatal("expected error to propagate")
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

func TestAudit_WritesOutputRecord(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	ctx := requestctx.WithRawPayload(context.Background(), []byte(`{"taskId":"t1"}`))
	_, _ = mw(okHandler)(ctx, req("acc1", "t1", "SHELL_SCRIPT"))

	if w.kind != audit.EventOutput {
		t.Errorf("kind = %q, want %q", w.kind, audit.EventOutput)
	}
	rec, ok := w.record.(audit.OutputRecord)
	if !ok {
		t.Fatalf("expected audit.OutputRecord, got %T", w.record)
	}
	if rec.AccountID != "acc1" {
		t.Errorf("AccountID = %q, want acc1", rec.AccountID)
	}
	if rec.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", rec.TaskID)
	}
	if rec.TaskTypeName != "SHELL_SCRIPT" {
		t.Errorf("TaskTypeName = %q, want SHELL_SCRIPT", rec.TaskTypeName)
	}
	if string(w.payload) != `{"taskId":"t1"}` {
		t.Errorf("payload = %s, want {\"taskId\":\"t1\"}", w.payload)
	}
}

func TestAudit_WritesOnError(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	_, _ = mw(errHandler)(context.Background(), req("acc1", "t1", "SHELL_SCRIPT"))

	if w.kind != audit.EventOutput {
		t.Errorf("expected audit record even on handler error, kind=%q", w.kind)
	}
}

func TestAudit_NoRawPayload(t *testing.T) {
	w := &captureWriter{}
	mw := Audit(w)

	// No requestctx payload set — should not panic, payload should be nil/empty.
	_, _ = mw(okHandler)(context.Background(), req("acc1", "t1", "SHELL_SCRIPT"))

	if w.kind != audit.EventOutput {
		t.Errorf("kind = %q, want %q", w.kind, audit.EventOutput)
	}
}
