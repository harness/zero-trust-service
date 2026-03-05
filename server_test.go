package zts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
)

func TestNewServer_DefaultRoutes(t *testing.T) {
	s := NewServer(WithMetrics(metrics.NewNoop()))

	req := httptest.NewRequest("POST", "/api/verify", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("/api/verify route not registered")
	}
}

func TestNewServer_OutputRoute(t *testing.T) {
	s := NewServer(WithMetrics(metrics.NewNoop()))

	req := httptest.NewRequest("POST", "/api/output", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("/api/output route not registered")
	}
}

func TestServer_Run_Shutdown(t *testing.T) {
	s := NewServer(WithMetrics(metrics.NewNoop()), WithPort(19876))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = s.Run(ctx)
}
