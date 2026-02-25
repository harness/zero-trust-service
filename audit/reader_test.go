package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
)

func setupReaderTest(t *testing.T) (string, *Reader) {
	t.Helper()
	dir := t.TempDir()

	// Create a writer to populate test data
	cfg := config.AuditConfig{Enabled: true, Dir: dir, MaxAgeDays: 30}
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	records := []Record{
		{ID: "r1", StartTs: now.UnixMilli(), EndTs: now.Add(5 * time.Millisecond).UnixMilli(), AccountID: "acc1", TaskID: "t1", TaskType: "SHELL_SCRIPT", Allowed: true},
		{ID: "r2", StartTs: now.Add(1 * time.Second).UnixMilli(), EndTs: now.Add(1*time.Second + 10*time.Millisecond).UnixMilli(), AccountID: "acc1", TaskID: "t2", TaskType: "HTTP_TASK", Allowed: false, Reason: "blocked by policy"},
		{ID: "r3", StartTs: now.Add(2 * time.Second).UnixMilli(), EndTs: now.Add(2*time.Second + 3*time.Millisecond).UnixMilli(), AccountID: "acc2", TaskID: "t3", TaskType: "SHELL_SCRIPT", Allowed: true},
	}

	for _, rec := range records {
		w.Write(rec, json.RawMessage(`{"delegateTaskId":"`+rec.TaskID+`"}`))
	}
	w.Close()

	reader := NewReader(dir)
	return dir, reader
}

func TestReader_List_AllRecords(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	resp, err := reader.List(ListRequest{
		FromMs: now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:   now.Add(1 * time.Hour).UnixMilli(),
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("expected 3 total, got %d", resp.Total)
	}
	if len(resp.Audits) != 3 {
		t.Errorf("expected 3 audits, got %d", len(resp.Audits))
	}
}

func TestReader_List_FilterByAccountID(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	resp, err := reader.List(ListRequest{
		FromMs:    now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:      now.Add(1 * time.Hour).UnixMilli(),
		AccountID: "acc1",
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 for acc1, got %d", resp.Total)
	}
}

func TestReader_List_FilterByTaskType(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	resp, err := reader.List(ListRequest{
		FromMs:   now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:     now.Add(1 * time.Hour).UnixMilli(),
		TaskType: "HTTP_TASK",
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 for HTTP_TASK, got %d", resp.Total)
	}
}

func TestReader_List_FilterByAllowed(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	denied := false
	resp, err := reader.List(ListRequest{
		FromMs:  now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:    now.Add(1 * time.Hour).UnixMilli(),
		Allowed: &denied,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 denied, got %d", resp.Total)
	}

	allowed := true
	resp2, err := reader.List(ListRequest{
		FromMs:  now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:    now.Add(1 * time.Hour).UnixMilli(),
		Allowed: &allowed,
		Limit:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.Total != 2 {
		t.Errorf("expected 2 allowed, got %d", resp2.Total)
	}
}

func TestReader_List_FilterByTaskID(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	resp, err := reader.List(ListRequest{
		FromMs: now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:   now.Add(1 * time.Hour).UnixMilli(),
		TaskID: "t2",
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 for t2, got %d", resp.Total)
	}
}

func TestReader_List_Pagination(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	// Page 1: limit=2, offset=0
	resp, err := reader.List(ListRequest{
		FromMs: now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:   now.Add(1 * time.Hour).UnixMilli(),
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Audits) != 2 {
		t.Errorf("expected 2 audits on page 1, got %d", len(resp.Audits))
	}
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}

	// Page 2: limit=2, offset=2
	resp2, err := reader.List(ListRequest{
		FromMs: now.Add(-1 * time.Hour).UnixMilli(),
		ToMs:   now.Add(1 * time.Hour).UnixMilli(),
		Limit:  2,
		Offset: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp2.Audits) != 1 {
		t.Errorf("expected 1 audit on page 2, got %d", len(resp2.Audits))
	}
}

func TestReader_List_OutOfRangeTimestamp(t *testing.T) {
	_, reader := setupReaderTest(t)

	// Query for a date range that doesn't match any records
	resp, err := reader.List(ListRequest{
		FromMs: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		ToMs:   time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected 0, got %d", resp.Total)
	}
}

func TestReader_GetPayload(t *testing.T) {
	_, reader := setupReaderTest(t)

	payload, err := reader.GetPayload("r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data map[string]string
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if data["delegateTaskId"] != "t1" {
		t.Errorf("expected t1, got %s", data["delegateTaskId"])
	}
}

func TestReader_GetPayload_NotFound(t *testing.T) {
	_, reader := setupReaderTest(t)

	_, err := reader.GetPayload("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing payload")
	}
}

func TestDatesToScan(t *testing.T) {
	from := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 17, 23, 59, 0, 0, time.UTC)

	dates := datesToScan(from, to)
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d: %v", len(dates), dates)
	}
	if dates[0] != "2026-02-15" || dates[1] != "2026-02-16" || dates[2] != "2026-02-17" {
		t.Errorf("unexpected dates: %v", dates)
	}
}

func TestDatesToScan_SameDay(t *testing.T) {
	from := time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 17, 20, 0, 0, 0, time.UTC)

	dates := datesToScan(from, to)
	if len(dates) != 1 {
		t.Fatalf("expected 1 date, got %d: %v", len(dates), dates)
	}
}

func TestMatchesFilters(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	rec := Record{
		StartTs:   now.UnixMilli(),
		AccountID: "acc1",
		TaskType:  "SHELL_SCRIPT",
		TaskID:    "t1",
		Allowed:   true,
	}

	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name  string
		req   ListRequest
		match bool
	}{
		{"all match", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli()}, true},
		{"account match", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), AccountID: "acc1"}, true},
		{"account mismatch", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), AccountID: "acc2"}, false},
		{"task type match", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), TaskType: "SHELL_SCRIPT"}, true},
		{"task type mismatch", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), TaskType: "OTHER"}, false},
		{"before range", ListRequest{FromMs: now.Add(1 * time.Hour).UnixMilli(), ToMs: now.Add(2 * time.Hour).UnixMilli()}, false},
		{"after range", ListRequest{FromMs: now.Add(-2 * time.Hour).UnixMilli(), ToMs: now.Add(-1 * time.Hour).UnixMilli()}, false},
		{"allowed match", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), Allowed: boolPtr(true)}, true},
		{"allowed mismatch", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), Allowed: boolPtr(false)}, false},
		{"allowed nil (no filter)", ListRequest{FromMs: now.Add(-1 * time.Hour).UnixMilli(), ToMs: now.Add(1 * time.Hour).UnixMilli(), Allowed: nil}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesFilters(rec, tc.req)
			if got != tc.match {
				t.Errorf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestScanMetadataFile_MissingFile(t *testing.T) {
	_, _, err := scanMetadataFile("/nonexistent/file.jsonl", ListRequest{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestScanMetadataFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	os.WriteFile(path, []byte("not-json\n{\"id\":\"ok\",\"startTs\":100,\"allowed\":true}\n"), 0600)

	records, total, err := scanMetadataFile(path, ListRequest{FromMs: 0, ToMs: 200, Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid line should be skipped, valid line should be returned
	if total != 1 {
		t.Errorf("expected 1 valid record, got %d", total)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}
