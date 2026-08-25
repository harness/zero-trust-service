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

package instrumented

import (
	"context"
	"testing"
)

func TestTracker_Record_And_Results(t *testing.T) {
	tracker := NewTracker()
	tracker.Record("v1", false)
	tracker.Record("v2", true)
	tracker.Record("v3", false)

	run, failed := tracker.Results()

	if len(run) != 3 {
		t.Fatalf("expected 3 validators run, got %d", len(run))
	}
	if run[0] != "v1" || run[1] != "v2" || run[2] != "v3" {
		t.Fatalf("unexpected validators: %v", run)
	}
	if failed != "v2" {
		t.Fatalf("expected failed=v2, got %q", failed)
	}
}

func TestTracker_FirstFailureWins(t *testing.T) {
	tracker := NewTracker()
	tracker.Record("a", true)
	tracker.Record("b", true)

	_, failed := tracker.Results()
	if failed != "a" {
		t.Fatalf("expected first failure 'a', got %q", failed)
	}
}

func TestTracker_NoFailure(t *testing.T) {
	tracker := NewTracker()
	tracker.Record("a", false)
	tracker.Record("b", false)

	run, failed := tracker.Results()
	if len(run) != 2 {
		t.Fatalf("expected 2, got %d", len(run))
	}
	if failed != "" {
		t.Fatalf("expected empty failed, got %q", failed)
	}
}

func TestWithTracker_And_TrackerFrom(t *testing.T) {
	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	got := TrackerFrom(ctx)
	if got != tracker {
		t.Fatal("expected same tracker from context")
	}
}

func TestTrackerFrom_NilContext(t *testing.T) {
	got := TrackerFrom(context.Background())
	if got != nil {
		t.Fatal("expected nil tracker from empty context")
	}
}
