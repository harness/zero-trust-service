package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
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
	defer w.Close()

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
	defer w.Close()

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
	defer w.Close()

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
	defer w.Close()

	now := time.Now().UTC()
	w.WriteEvent(audit.EventVerify, audit.Record{ID: "r1", StartTime: now, Allowed: true}, json.RawMessage(`{}`))

	w.mu.Lock()
	for _, h := range w.files {
		h.file.Close()
	}
	w.mu.Unlock()

	os.RemoveAll(filepath.Join(dir, "metadata"))

	w.WriteEvent(audit.EventVerify, audit.Record{ID: "r2", StartTime: now, Allowed: true}, json.RawMessage(`{}`))

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", date, "verify.jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata dir was not recreated after self-heal")
	}
}

func TestWriter_DateRotation(t *testing.T) {
	w, dir := newTestWriter(t)
	defer w.Close()

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

func TestWriter_Cleanup(t *testing.T) {
	w, dir := newTestWriter(t)
	w.cfg.MaxAgeDays = 1
	defer w.Close()

	oldDate := time.Now().AddDate(0, 0, -40).UTC().Format("2006-01-02")

	oldMetaDir := filepath.Join(dir, "metadata", oldDate)
	os.MkdirAll(oldMetaDir, 0700)
	os.WriteFile(filepath.Join(oldMetaDir, "verify.jsonl"), []byte(`{"id":"old"}`+"\n"), 0600)

	oldPayloadDir := filepath.Join(dir, "payloads", oldDate)
	os.MkdirAll(filepath.Join(oldPayloadDir, "verify"), 0700)
	os.WriteFile(filepath.Join(oldPayloadDir, "verify", "old.json"), []byte(`{}`), 0600)

	todayDate := time.Now().UTC().Format("2006-01-02")
	recentMetaDir := filepath.Join(dir, "metadata", todayDate)
	os.MkdirAll(recentMetaDir, 0700)
	os.WriteFile(filepath.Join(recentMetaDir, "verify.jsonl"), []byte(`{"id":"recent"}`+"\n"), 0600)

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
