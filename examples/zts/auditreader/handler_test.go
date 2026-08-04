package auditreader

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
	auditfile "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit/file"
	"github.com/go-chi/chi/v5"
)

func setupHandlerTest(t *testing.T) (*Handler, *chi.Mux) {
	t.Helper()
	dir := t.TempDir()

	cfg := auditfile.Config{Dir: dir, MaxAgeDays: 30}
	w, err := auditfile.NewWriter(cfg)
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

	_ = w.Close()

	reader := auditfile.NewReader(dir)
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

// validRange returns the from/to query params covering the seeded test records.
func validRange() string {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	from := now.Add(-1 * time.Hour).UnixMilli()
	to := now.Add(1 * time.Hour).UnixMilli()
	return "from=" + i64(from) + "&to=" + i64(to)
}

func TestHandleList_OK(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantTotal int
		wantKind  string
	}{
		{"verify", validRange(), 2, audit.EventVerify},
		{"output", validRange() + "&kind=output", 1, audit.EventOutput},
		{"filter by account", validRange() + "&account_id=acc1", 1, audit.EventVerify},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, router := setupHandlerTest(t)
			req := httptest.NewRequest("GET", "/api/audits?"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var resp auditfile.ListResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
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

func TestHandleList_BadRequest(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"missing from", ""},
		{"invalid from", "from=abc&to=123"},
		{"invalid to", "from=123&to=abc"},
		{"to before from", "from=200&to=100"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, router := setupHandlerTest(t)
			req := httptest.NewRequest("GET", "/api/audits?"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleList_LimitCapped(t *testing.T) {
	_, router := setupHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/audits?"+validRange()+"&limit=9999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp auditfile.ListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Limit != maxLimit {
		t.Errorf("expected limit capped to %d, got %d", maxLimit, resp.Limit)
	}
}

func TestHandleGetPayload_Success(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantKey string
		wantVal string
	}{
		{"verify payload", "h1", "delegateTaskId", "t1"},
		{"output payload", "ho1", "taskOutput", "data"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, router := setupHandlerTest(t)
			req := httptest.NewRequest("GET", "/api/audits/"+tc.id+"/payload", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var data map[string]string
			_ = json.Unmarshal(w.Body.Bytes(), &data)
			if data[tc.wantKey] != tc.wantVal {
				t.Errorf("expected %s=%s, got %s", tc.wantKey, tc.wantVal, data[tc.wantKey])
			}
		})
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
	_ = os.MkdirAll(filepath.Join(dir, "metadata"), 0700)

	reader := auditfile.NewReader(dir)
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
