package zts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
)

func TestNewServer_DefaultRoutes(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m))

	// Verify /api/verify endpoint exists
	req := httptest.NewRequest("POST", "/api/verify", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	// Should get 400 (bad request) not 404 (not found)
	if w.Code == http.StatusNotFound {
		t.Fatal("/api/verify route not registered")
	}
}

func TestNewServer_MetricsEndpoint(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m))

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", w.Code)
	}
}

func TestNewServer_AuditRoutesRegistered(t *testing.T) {
	m := testMetricsForRoot()
	dir := t.TempDir()
	cfg := config.AuditConfig{Enabled: true, Dir: dir, MaxAgeDays: 30}
	aw, err := audit.NewWriter(cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer aw.Close()

	reader := audit.NewReader(dir)
	ah := audit.NewHandler(reader)

	s := NewServer(
		WithMetrics(m),
		WithAuditWriter(aw),
		WithAuditHandler(ah),
	)

	// /api/audits should return 400 (missing params) not 404
	req := httptest.NewRequest("GET", "/api/audits", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("/api/audits route not registered when audit is enabled")
	}
}

func TestNewServer_NoAuditRoutes(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m))

	req := httptest.NewRequest("GET", "/api/audits", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 404 or 405 when audit is disabled, got %d", w.Code)
	}
}

func TestServer_Run_Shutdown(t *testing.T) {
	m := testMetricsForRoot()
	s := NewServer(WithMetrics(m), WithPort(19876))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run will start the server and then ctx will be cancelled
	_ = s.Run(ctx)
	// If we get here without hanging, the shutdown works
}
