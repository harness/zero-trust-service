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
	"errors"
	"testing"

	"github.com/harness/zero-trust-service/metrics"
	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
)

func TestInstrumented_Pass(t *testing.T) {
	m := metrics.NewNoop()
	inner := verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Wrap("test_v", inner, m)

	err := v.Handle(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestInstrumented_Fail_RecordsBlocked(t *testing.T) {
	m := metrics.NewNoop()
	inner := verifier.From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("denied")
	})
	v := Wrap("test_v", inner, m)

	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			AccountID: "acc1",
			TaskDetails: &types.TaskDetails{
				TaskType: "SHELL_SCRIPT",
			},
		},
	}
	err := v.Handle(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstrumented_RecordsInTracker(t *testing.T) {
	m := metrics.NewNoop()
	inner := verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	v := Wrap("my_validator", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	run, failed := tracker.Results()
	if len(run) != 1 || run[0] != "my_validator" {
		t.Fatalf("expected [my_validator], got %v", run)
	}
	if failed != "" {
		t.Fatalf("expected no failure, got %q", failed)
	}
}

func TestInstrumented_RecordsFailureInTracker(t *testing.T) {
	m := metrics.NewNoop()
	inner := verifier.From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("nope")
	})
	v := Wrap("bad_v", inner, m)

	tracker := NewTracker()
	ctx := WithTracker(context.Background(), tracker)

	_ = v.Handle(ctx, types.VerifyRequest{})

	_, failed := tracker.Results()
	if failed != "bad_v" {
		t.Fatalf("expected bad_v, got %q", failed)
	}
}
