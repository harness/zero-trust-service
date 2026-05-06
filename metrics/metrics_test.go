package metrics

import "testing"

func TestNewNoop_ImplementsEmitter(t *testing.T) {
	var _ Emitter = NewNoop()
}

func TestNewNoop_OperationsDoNotPanic(t *testing.T) {
	m := NewNoop()
	m.Counter("test_counter", 1, Dimension{Key: "status", Value: "success"}, Dimension{Key: "account_id", Value: "acc"})
	m.Histogram("test_hist", 0.5, Dimension{Key: "status", Value: "success"})
	m.Gauge("test_gauge", 3, Dimension{Key: "scope", Value: "global"})
}

func TestDimension(t *testing.T) {
	d := Dimension{Key: "key", Value: "val"}
	if d.Key != "key" || d.Value != "val" {
		t.Errorf("expected {key, val}, got {%s, %s}", d.Key, d.Value)
	}
}

func TestDim(t *testing.T) {
	d := Dim("key", "val")
	if d.Key != "key" || d.Value != "val" {
		t.Errorf("expected {key, val}, got {%s, %s}", d.Key, d.Value)
	}
}
