package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
)

func setupReaderTest(t *testing.T) (string, *Reader) {
	t.Helper()
	dir := t.TempDir()

	cfg := Config{Dir: dir, MaxAgeDays: 30}
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	verifyRecords := []audit.Record{
		{ID: "r1", StartTime: now, EndTime: now.Add(5 * time.Millisecond), AccountID: "acc1", TaskID: "t1", TaskType: "SHELL_SCRIPT", Allowed: true},
		{ID: "r2", StartTime: now.Add(1 * time.Second), EndTime: now.Add(1*time.Second + 10*time.Millisecond), AccountID: "acc1", TaskID: "t2", TaskType: "HTTP_TASK", Allowed: false, Reason: "blocked by policy"},
		{ID: "r3", StartTime: now.Add(2 * time.Second), EndTime: now.Add(2*time.Second + 3*time.Millisecond), AccountID: "acc2", TaskID: "t3", TaskType: "SHELL_SCRIPT", Allowed: true},
	}
	for _, rec := range verifyRecords {
		w.WriteEvent(audit.EventVerify, rec, json.RawMessage(`{"delegateTaskId":"`+rec.TaskID+`"}`))
	}

	outputRecords := []audit.OutputRecord{
		{ID: "o1", Timestamp: now.UnixMilli(), AccountID: "acc1", TaskID: "t1", TaskTypeName: "SHELL_SCRIPT", ResponseCode: "OK"},
		{ID: "o2", Timestamp: now.Add(1 * time.Second).UnixMilli(), AccountID: "acc2", TaskID: "t4", TaskTypeName: "HTTP_TASK", ResponseCode: "FAILED"},
	}
	for _, rec := range outputRecords {
		w.WriteEvent(audit.EventOutput, rec, json.RawMessage(`{"taskOutput":"data"}`))
	}

	_ = w.Close()

	reader := NewReader(dir)
	return dir, reader
}

func TestReader_List_Filters(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	denied, allowed := false, true

	tests := []struct {
		name      string
		req       ListRequest
		wantTotal int
		wantKind  string
	}{
		{"all verify records", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, Limit: 100}, 3, audit.EventVerify},
		{"all output records", ListRequest{Kind: audit.EventOutput, FromTime: from, ToTime: to, Limit: 100}, 2, audit.EventOutput},
		{"default kind is verify", ListRequest{FromTime: from, ToTime: to, Limit: 100}, 3, audit.EventVerify},
		{"filter verify by account", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, AccountID: "acc1", Limit: 100}, 2, audit.EventVerify},
		{"filter output by account", ListRequest{Kind: audit.EventOutput, FromTime: from, ToTime: to, AccountID: "acc1", Limit: 100}, 1, audit.EventOutput},
		{"filter by task type", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, TaskType: "HTTP_TASK", Limit: 100}, 1, audit.EventVerify},
		{"filter by task id", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, TaskID: "t2", Limit: 100}, 1, audit.EventVerify},
		{"filter by denied", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, Allowed: &denied, Limit: 100}, 1, audit.EventVerify},
		{"filter by allowed", ListRequest{Kind: audit.EventVerify, FromTime: from, ToTime: to, Allowed: &allowed, Limit: 100}, 2, audit.EventVerify},
		{"out of range timestamp", ListRequest{Kind: audit.EventVerify, FromTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ToTime: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Limit: 100}, 0, audit.EventVerify},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, reader := setupReaderTest(t)
			resp, err := reader.List(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Total != tc.wantTotal {
				t.Errorf("expected total %d, got %d", tc.wantTotal, resp.Total)
			}
			if resp.Kind != tc.wantKind {
				t.Errorf("expected kind %s, got %s", tc.wantKind, resp.Kind)
			}
		})
	}
}

func TestReader_List_Pagination(t *testing.T) {
	_, reader := setupReaderTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	resp, err := reader.List(ListRequest{
		Kind:     audit.EventVerify,
		FromTime: now.Add(-1 * time.Hour),
		ToTime:   now.Add(1 * time.Hour),
		Limit:    2,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	audits := resp.Audits.([]audit.Record)
	if len(audits) != 2 {
		t.Errorf("expected 2 audits on page 1, got %d", len(audits))
	}
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}

	resp2, err := reader.List(ListRequest{
		Kind:     audit.EventVerify,
		FromTime: now.Add(-1 * time.Hour),
		ToTime:   now.Add(1 * time.Hour),
		Limit:    2,
		Offset:   2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	audits2 := resp2.Audits.([]audit.Record)
	if len(audits2) != 1 {
		t.Errorf("expected 1 audit on page 2, got %d", len(audits2))
	}
}

func TestReader_GetPayload_Verify(t *testing.T) {
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

func TestReader_GetPayload_Output(t *testing.T) {
	_, reader := setupReaderTest(t)

	payload, err := reader.GetPayload("o1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data map[string]string
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if data["taskOutput"] != "data" {
		t.Errorf("expected data, got %s", data["taskOutput"])
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

func TestMatchesVerifyFilters(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	rec := audit.Record{
		StartTime: now,
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
		{"all match", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour)}, true},
		{"account match", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), AccountID: "acc1"}, true},
		{"account mismatch", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), AccountID: "acc2"}, false},
		{"task type match", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), TaskType: "SHELL_SCRIPT"}, true},
		{"task type mismatch", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), TaskType: "OTHER"}, false},
		{"before range", ListRequest{FromTime: now.Add(1 * time.Hour), ToTime: now.Add(2 * time.Hour)}, false},
		{"after range", ListRequest{FromTime: now.Add(-2 * time.Hour), ToTime: now.Add(-1 * time.Hour)}, false},
		{"allowed match", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), Allowed: boolPtr(true)}, true},
		{"allowed mismatch", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), Allowed: boolPtr(false)}, false},
		{"allowed nil (no filter)", ListRequest{FromTime: now.Add(-1 * time.Hour), ToTime: now.Add(1 * time.Hour), Allowed: nil}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesVerifyFilters(rec, tc.req)
			if got != tc.match {
				t.Errorf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestScanVerifyFile_MissingFile(t *testing.T) {
	_, _, err := scanVerifyFile("/nonexistent/file.jsonl", ListRequest{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestScanVerifyFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	_ = os.WriteFile(path, []byte("not-json\n{\"id\":\"ok\",\"startTime\":\"1970-01-01T00:01:40Z\",\"allowed\":true}\n"), 0600)

	records, total, err := scanVerifyFile(path, ListRequest{FromTime: time.Unix(0, 0), ToTime: time.Unix(200, 0), Limit: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 valid record, got %d", total)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}
