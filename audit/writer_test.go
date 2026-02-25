package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
)

func newTestWriter(t *testing.T) (*Writer, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.AuditConfig{
		Enabled:    true,
		Dir:        dir,
		MaxAgeDays: 30,
	}
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	return w, dir
}

func TestWriter_Write_CreatesMetadataAndPayload(t *testing.T) {
	w, dir := newTestWriter(t)
	defer w.Close()

	now := time.Now().UTC()
	record := Record{
		ID:        "test-001",
		StartTs:   now.UnixMilli(),
		EndTs:     now.Add(10 * time.Millisecond).UnixMilli(),
		AccountID: "acc1",
		TaskID:    "task-1",
		TaskType:  "SHELL_SCRIPT",
		Allowed:   true,
	}
	payload := json.RawMessage(`{"delegateTaskId":"task-1"}`)

	w.Write(record, payload)

	// Check metadata file exists
	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", "audit-"+date+".jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatalf("metadata file not created: %s", metaPath)
	}

	// Read and verify metadata
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var rec Record
	if err := json.Unmarshal(data[:len(data)-1], &rec); err != nil { // -1 to strip trailing newline
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if rec.ID != "test-001" {
		t.Errorf("expected id test-001, got %s", rec.ID)
	}

	// Check payload file exists
	payloadPath := filepath.Join(dir, "payloads", date, "test-001.json")
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		t.Fatalf("payload file not created: %s", payloadPath)
	}
	pData, _ := os.ReadFile(payloadPath)
	if string(pData) != `{"delegateTaskId":"task-1"}` {
		t.Errorf("unexpected payload: %s", pData)
	}
}

func TestWriter_Write_MultipleRecordsSameDay(t *testing.T) {
	w, dir := newTestWriter(t)
	defer w.Close()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		record := Record{
			ID:      "rec-" + string(rune('a'+i)),
			StartTs: now.UnixMilli(),
			Allowed: true,
		}
		w.Write(record, json.RawMessage(`{}`))
	}

	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", "audit-"+date+".jsonl")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	// Should have 3 lines
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

	// Write something to open a file handle
	now := time.Now().UTC()
	w.Write(Record{ID: "x", StartTs: now.UnixMilli(), Allowed: true}, json.RawMessage(`{}`))

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Double close should be safe (returns nil because metaFile is already nil)
	if err := w.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestWriter_SelfHeal_StaleFileHandle(t *testing.T) {
	w, dir := newTestWriter(t)
	defer w.Close()

	now := time.Now().UTC()

	// Write first record to establish file handle
	w.Write(Record{ID: "r1", StartTs: now.UnixMilli(), Allowed: true}, json.RawMessage(`{}`))

	// Simulate a stale file handle by closing it behind the writer's back
	w.mu.Lock()
	if w.metaFile != nil {
		w.metaFile.Close()
	}
	w.mu.Unlock()

	// Delete the metadata directory to force a full re-create
	os.RemoveAll(filepath.Join(dir, "metadata"))

	// Write again — should self-heal: detect write failure, reopen, and recreate dir
	w.Write(Record{ID: "r2", StartTs: now.UnixMilli(), Allowed: true}, json.RawMessage(`{}`))

	// Verify r2 was written
	date := now.Format("2006-01-02")
	metaPath := filepath.Join(dir, "metadata", "audit-"+date+".jsonl")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Fatal("metadata dir was not recreated after self-heal")
	}
}

func TestWriter_DateRotation(t *testing.T) {
	w, dir := newTestWriter(t)
	defer w.Close()

	// Write a record for "yesterday" by using a past timestamp
	yesterday := time.Now().AddDate(0, 0, -1).UTC()
	w.Write(Record{ID: "y1", StartTs: yesterday.UnixMilli(), Allowed: true}, json.RawMessage(`{}`))

	// Write a record for "today"
	today := time.Now().UTC()
	w.Write(Record{ID: "t1", StartTs: today.UnixMilli(), Allowed: true}, json.RawMessage(`{}`))

	// Both files should exist
	yDate := yesterday.Format("2006-01-02")
	tDate := today.Format("2006-01-02")

	yPath := filepath.Join(dir, "metadata", "audit-"+yDate+".jsonl")
	tPath := filepath.Join(dir, "metadata", "audit-"+tDate+".jsonl")

	if _, err := os.Stat(yPath); os.IsNotExist(err) {
		t.Fatalf("yesterday metadata file not created: %s", yPath)
	}
	if _, err := os.Stat(tPath); os.IsNotExist(err) {
		t.Fatalf("today metadata file not created: %s", tPath)
	}
}

func TestWriter_RunCleanup(t *testing.T) {
	w, dir := newTestWriter(t)
	w.cfg.MaxAgeDays = 1
	defer w.Close()

	// Create old metadata file (40 days ago)
	oldDate := time.Now().AddDate(0, 0, -40).UTC().Format("2006-01-02")
	metaDir := filepath.Join(dir, "metadata")
	os.MkdirAll(metaDir, 0700)
	oldMeta := filepath.Join(metaDir, "audit-"+oldDate+".jsonl")
	os.WriteFile(oldMeta, []byte(`{"id":"old"}`+"\n"), 0600)

	// Create old payload dir
	oldPayloadDir := filepath.Join(dir, "payloads", oldDate)
	os.MkdirAll(oldPayloadDir, 0700)
	os.WriteFile(filepath.Join(oldPayloadDir, "old.json"), []byte(`{}`), 0600)

	// Create recent file (today)
	todayDate := time.Now().UTC().Format("2006-01-02")
	recentMeta := filepath.Join(metaDir, "audit-"+todayDate+".jsonl")
	os.WriteFile(recentMeta, []byte(`{"id":"recent"}`+"\n"), 0600)

	w.RunCleanup()

	// Old files should be gone
	if _, err := os.Stat(oldMeta); !os.IsNotExist(err) {
		t.Error("old metadata file should have been cleaned up")
	}
	if _, err := os.Stat(oldPayloadDir); !os.IsNotExist(err) {
		t.Error("old payload dir should have been cleaned up")
	}

	// Recent file should remain
	if _, err := os.Stat(recentMeta); os.IsNotExist(err) {
		t.Error("recent metadata file should NOT have been cleaned up")
	}
}
