package file

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"github.com/go-chi/chi/v5"
)

func setupHandlerTest(t *testing.T) (*Handler, *chi.Mux) {
	t.Helper()
	dir := t.TempDir()

	cfg := Config{Dir: dir, MaxAgeDays: 30}
	w, err := NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	verifyRecords := []audit.Record{
		{ID: "h1", StartTime: now, EndTime: now.Add(5 * time.Millisecond), AccountID: "acc1", TaskID: "t1", TaskType: "SHELL_SCRIPT", Allowed: true},
		{ID: "h2", StartTime: now.Add(1 * time.Second), EndTime: now.Add(1*time.Second + 10*time.Millisecond), AccountID: "acc2", TaskID: "t2", TaskType: "HTTP_TASK", Allowed: false, Reason: "blocked"},
	}
	for _, rec := range verifyRecords {
		w.WriteEvent(audit.EventVerify, rec, json.RawMessage(`{"delegateTaskId":"`+rec.TaskID+`"}`))
	}

	outputRecords := []audit.OutputRecord{
		{ID: "ho1", Timestamp: now.UnixMilli(), AccountID: "acc1", TaskID: "t1", TaskTypeName: "SHELL_SCRIPT", ResponseCode: "OK"},
	}
	for _, rec := range outputRecords {
		w.WriteEvent(audit.EventOutput, rec, json.RawMessage(`{"taskOutput":"data"}`))
	}

	w.Close()

	reader := NewReader(dir)
	handler := NewHandler(reader)

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		handler.RegisterRoutes(r)
	})

	return handler, r
}

func i64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestHandleList_Verify(t *testing.T) {
	_, router := setupHandlerTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour).UnixMilli()
	to := now.Add(1 * time.Hour).UnixMilli()

	req := httptest.NewRequest("GET", "/api/audits?from="+i64(from)+"&to="+i64(to), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 total, got %d", resp.Total)
	}
	if resp.Kind != audit.EventVerify {
		t.Errorf("expected kind verify, got %s", resp.Kind)
	}
}

func TestHandleList_Output(t *testing.T) {
	_, router := setupHandlerTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour).UnixMilli()
	to := now.Add(1 * time.Hour).UnixMilli()

	req := httptest.NewRequest("GET", "/api/audits?from="+i64(from)+"&to="+i64(to)+"&kind=output", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 output total, got %d", resp.Total)
	}
	if resp.Kind != audit.EventOutput {
		t.Errorf("expected kind output, got %s", resp.Kind)
	}
}

func TestHandleList_FilterByAccountID(t *testing.T) {
	_, router := setupHandlerTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour).UnixMilli()
	to := now.Add(1 * time.Hour).UnixMilli()

	req := httptest.NewRequest("GET", "/api/audits?from="+i64(from)+"&to="+i64(to)+"&account_id=acc1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("expected 1 for acc1, got %d", resp.Total)
	}
}

func TestHandleList_MissingFrom(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_InvalidFrom(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits?from=abc&to=123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_InvalidTo(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits?from=123&to=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_ToBeforeFrom(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits?from=200&to=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_LimitCapped(t *testing.T) {
	_, router := setupHandlerTest(t)

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour).UnixMilli()
	to := now.Add(1 * time.Hour).UnixMilli()

	req := httptest.NewRequest("GET", "/api/audits?from="+i64(from)+"&to="+i64(to)+"&limit=9999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Limit != maxLimit {
		t.Errorf("expected limit capped to %d, got %d", maxLimit, resp.Limit)
	}
}

func TestHandleGetPayload_Success(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits/h1/payload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var data map[string]string
	json.Unmarshal(w.Body.Bytes(), &data)
	if data["delegateTaskId"] != "t1" {
		t.Errorf("expected t1, got %s", data["delegateTaskId"])
	}
}

func TestHandleGetPayload_OutputPayload(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits/ho1/payload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var data map[string]string
	json.Unmarshal(w.Body.Bytes(), &data)
	if data["taskOutput"] != "data" {
		t.Errorf("expected data, got %s", data["taskOutput"])
	}
}

func TestHandleGetPayload_NotFound(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits/nonexistent/payload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetPayload_EmptyPayloadsDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "metadata"), 0700)

	reader := NewReader(dir)
	handler := NewHandler(reader)

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		handler.RegisterRoutes(r)
	})

	req := httptest.NewRequest("GET", "/api/audits/missing/payload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
