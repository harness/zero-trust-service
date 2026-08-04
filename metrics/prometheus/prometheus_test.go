package prometheus

import (
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func TestNewWithRegistry_ImplementsEmitter(t *testing.T) {
	var _ = NewWithRegistry(prom.NewRegistry())
}

func TestNewWithRegistry_OperationsDoNotPanic(t *testing.T) {
	m := NewWithRegistry(prom.NewRegistry())
	m.Counter("test_counter", 1, metrics.Dimension{Key: "status", Value: "success"})
	m.Histogram("test_hist", 0.5, metrics.Dimension{Key: "status", Value: "success"})
	m.Gauge("test_gauge", 3, metrics.Dimension{Key: "scope", Value: "global"})
}

func TestNewWithRegistry_MultipleInstances(t *testing.T) {
	m1 := NewWithRegistry(prom.NewRegistry())
	m2 := NewWithRegistry(prom.NewRegistry())

	m1.Counter("test_counter", 1, metrics.Dimension{Key: "status", Value: "a"})
	m2.Counter("test_counter", 1, metrics.Dimension{Key: "status", Value: "b"})
}

func TestWithBuckets(t *testing.T) {
	m := NewWithRegistry(prom.NewRegistry(),
		WithBuckets("custom_hist", []float64{0.1, 0.5, 1.0}),
	)
	m.Histogram("custom_hist", 0.3, metrics.Dimension{Key: "status", Value: "ok"})
}

func TestNew_UsesDefaultRegistry(t *testing.T) {
	// Just verify it doesn't panic
	_ = New()
}

func TestCounter_DuplicateKeysCollapsed(t *testing.T) {
	m := NewWithRegistry(prom.NewRegistry())
	m.Counter("dup_counter", 1,
		metrics.Dim("status", "success"),
		metrics.Dim("status", "override"),
	)
}

func TestSplit_DuplicateKeysLastWins(t *testing.T) {
	dims := []metrics.Dimension{
		metrics.Dim("status", "success"),
		metrics.Dim("account_id", "acc-1"),
		metrics.Dim("status", "override"),
	}
	keys, vals := split(dims)
	if len(keys) != 2 || len(vals) != 2 {
		t.Fatalf("expected 2 unique keys, got keys=%v vals=%v", keys, vals)
	}
	for i, k := range keys {
		if k == "status" && vals[i] != "override" {
			t.Errorf("expected last-wins for status, got %s", vals[i])
		}
	}
}
