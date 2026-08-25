// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import "testing"

func TestNewNoop_ImplementsEmitter(t *testing.T) {
	var _ = NewNoop()
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
