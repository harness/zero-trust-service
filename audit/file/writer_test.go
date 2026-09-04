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

package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harness/zero-trust-service/audit"
)

func newTestWriter(t *testing.T) (*Writer, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{Dir: dir, MaxAgeDays: 30}
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	return w, dir
}

func TestWriter_WriteEvent_Verify(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	record := audit.Record{
		ID:        "test-001",
		StartTime: now,
		EndTime:   now.Add(10 * time.Millisecond),
		AccountID: "acc1",
		TaskID:    "task-1",
		TaskType:  "SHELL_SCRIPT",
		Allowed:   true,
	}
	payload := json.RawMessage(`{"delegateTaskId":"task-1"}`)

	w.WriteEvent(audit.EventVerify, record, payload)

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", date, "verify.jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatalf("verify metadata file not created: %s", metaPath)
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var rec audit.Record
	if err := json.Unmarshal(data[:len(data)-1], &rec); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if rec.ID != "test-001" {
		t.Errorf("expected id test-001, got %s", rec.ID)
	}

	payloadPath := filepath.Join(dir, "payloads", date, "verify", "test-001.json")
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		t.Fatalf("payload file not created: %s", payloadPath)
	}
	pData, _ := os.ReadFile(payloadPath)
	if string(pData) != `{"delegateTaskId":"task-1"}` {
		t.Errorf("unexpected payload: %s", pData)
	}
}

func TestWriter_WriteEvent_Output(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	record := audit.OutputRecord{
		ID:           "out-001",
		Timestamp:    now.UnixMilli(),
		AccountID:    "acc1",
		TaskID:       "task-1",
		TaskTypeName: "SHELL_SCRIPT",
		ResponseCode: "OK",
	}
	payload := json.RawMessage(`{"taskOutput":"data"}`)

	w.WriteEvent(audit.EventOutput, record, payload)

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", date, "output.jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatalf("output metadata file not created: %s", metaPath)
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var rec audit.OutputRecord
	if err := json.Unmarshal(data[:len(data)-1], &rec); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if rec.ID != "out-001" {
		t.Errorf("expected id out-001, got %s", rec.ID)
	}

	payloadPath := filepath.Join(dir, "payloads", date, "output", "out-001.json")
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		t.Fatalf("payload file not created: %s", payloadPath)
	}
}

func TestWriter_WriteEvent_MultipleRecordsSameDay(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		record := audit.Record{
			ID:        "rec-" + string(rune('a'+i)),
			StartTime: now,
			Allowed:   true,
		}
		w.WriteEvent(audit.EventVerify, record, json.RawMessage(`{}`))
	}

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", date, "verify.jsonl")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestWriter_Close(t *testing.T) {
	w, _ := newTestWriter(t)

	now := time.Now().UTC()
	w.WriteEvent(audit.EventVerify, audit.Record{ID: "x", StartTime: now, Allowed: true}, json.RawMessage(`{}`))

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestWriter_SelfHeal_StaleFileHandle(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	now := time.Now().UTC()
	w.WriteEvent(audit.EventVerify, audit.Record{ID: "r1", StartTime: now, Allowed: true}, json.RawMessage(`{}`))

	w.mu.Lock()
	for _, h := range w.files {
		_ = h.file.Close()
	}
	w.mu.Unlock()

	_ = os.RemoveAll(filepath.Join(dir, "metadata"))

	w.WriteEvent(audit.EventVerify, audit.Record{ID: "r2", StartTime: now, Allowed: true}, json.RawMessage(`{}`))

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", date, "verify.jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata dir was not recreated after self-heal")
	}
}

func TestWriter_DateRotation(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	yesterday := time.Now().AddDate(0, 0, -1).UTC()
	w.WriteEvent(audit.EventVerify, audit.Record{ID: "y1", StartTime: yesterday, Allowed: true}, json.RawMessage(`{}`))

	today := time.Now().UTC()
	w.WriteEvent(audit.EventVerify, audit.Record{ID: "t1", StartTime: today, Allowed: true}, json.RawMessage(`{}`))

	yDate := yesterday.Format("2006-01-02")
	tDate := today.Format("2006-01-02")

	yPath := filepath.Join(dir, "metadata", yDate, "verify.jsonl")
	tPath := filepath.Join(dir, "metadata", tDate, "verify.jsonl")

	if _, err := os.Stat(yPath); os.IsNotExist(err) {
		t.Fatalf("yesterday metadata file not created: %s", yPath)
	}
	if _, err := os.Stat(tPath); os.IsNotExist(err) {
		t.Fatalf("today metadata file not created: %s", tPath)
	}
}

func TestWriter_Start_ContextCancel(t *testing.T) {
	w, _ := newTestWriter(t)
	defer func() { _ = w.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestCleanDateDirs_SkipsNonDirs(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	metaDir := filepath.Join(dir, "metadata")
	_ = os.MkdirAll(metaDir, 0700)
	// Write a plain file (not a directory) — cleanDateDirs should skip it without panic.
	_ = os.WriteFile(filepath.Join(metaDir, "not-a-dir.txt"), []byte("x"), 0600)

	cutoff := time.Now().AddDate(0, 0, 1) // future cutoff — would delete everything if it ran
	w.cleanDateDirs(metaDir, cutoff)

	if _, err := os.Stat(filepath.Join(metaDir, "not-a-dir.txt")); os.IsNotExist(err) {
		t.Error("plain file should not have been removed by cleanDateDirs")
	}
}

func TestNewWriter_DefaultDir(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(Config{Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.cfg.MaxAgeDays != 30 {
		t.Errorf("expected MaxAgeDays default 30, got %d", w.cfg.MaxAgeDays)
	}
	_ = w.Close()
}

func TestNewWriter_Error(t *testing.T) {
	// Use a file as Dir so MkdirAll fails.
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	_, err = NewWriter(Config{Dir: f.Name()})
	if err == nil {
		t.Fatal("expected error when dir is a file")
	}
}

func TestWritePayload_BadDir(t *testing.T) {
	w, dir := newTestWriter(t)
	defer func() { _ = w.Close() }()

	// Replace the payloads dir with a file so writePayload's MkdirAll fails.
	payloadsDir := filepath.Join(dir, "payloads")
	_ = os.RemoveAll(payloadsDir)
	f, err := os.Create(payloadsDir)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	now := time.Now().UTC()
	// writePayload's MkdirAll fails — should log, not panic.
	w.WriteEvent(audit.EventVerify,
		audit.Record{ID: "x", StartTime: now, Allowed: true},
		json.RawMessage(`{}`),
	)
}

func TestWriter_Cleanup(t *testing.T) {
	w, dir := newTestWriter(t)
	w.cfg.MaxAgeDays = 1
	defer func() { _ = w.Close() }()

	oldDate := time.Now().AddDate(0, 0, -40).UTC().Format("2006-01-02")

	oldMetaDir := filepath.Join(dir, "metadata", oldDate)
	_ = os.MkdirAll(oldMetaDir, 0700)
	_ = os.WriteFile(filepath.Join(oldMetaDir, "verify.jsonl"), []byte(`{"id":"old"}`+"\n"), 0600)

	oldPayloadDir := filepath.Join(dir, "payloads", oldDate)
	_ = os.MkdirAll(filepath.Join(oldPayloadDir, "verify"), 0700)
	_ = os.WriteFile(filepath.Join(oldPayloadDir, "verify", "old.json"), []byte(`{}`), 0600)

	todayDate := time.Now().UTC().Format("2006-01-02")
	recentMetaDir := filepath.Join(dir, "metadata", todayDate)
	_ = os.MkdirAll(recentMetaDir, 0700)
	_ = os.WriteFile(filepath.Join(recentMetaDir, "verify.jsonl"), []byte(`{"id":"recent"}`+"\n"), 0600)

	w.runCleanup()

	if _, err := os.Stat(oldMetaDir); !os.IsNotExist(err) {
		t.Error("old metadata dir should have been cleaned up")
	}
	if _, err := os.Stat(oldPayloadDir); !os.IsNotExist(err) {
		t.Error("old payload dir should have been cleaned up")
	}
	if _, err := os.Stat(recentMetaDir); os.IsNotExist(err) {
		t.Error("recent metadata dir should NOT have been cleaned up")
	}
}
