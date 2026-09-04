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

package verifier

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/zero-trust-service/types"
)

func TestChain_AllPass(t *testing.T) {
	noop := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	chain := Chain(noop, noop, noop)

	if err := chain.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestChain_StopsOnFirstError(t *testing.T) {
	calls := 0
	counter := From(func(_ context.Context, _ types.VerifyRequest) error {
		calls++
		return nil
	})
	fail := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("blocked")
	})
	afterFail := From(func(_ context.Context, _ types.VerifyRequest) error {
		t.Fatal("should not be called after failure")
		return nil
	})

	chain := Chain(counter, fail, afterFail)
	err := chain.Handle(context.Background(), types.VerifyRequest{})

	if err == nil || err.Error() != "blocked" {
		t.Fatalf("expected 'blocked' error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected counter called once, got %d", calls)
	}
}

func TestChain_Empty(t *testing.T) {
	chain := Chain()
	if err := chain.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("empty chain should pass, got %v", err)
	}
}

func TestChain_ExecutionOrder(t *testing.T) {
	var order []int
	mk := func(id int) Interface {
		return From(func(_ context.Context, _ types.VerifyRequest) error {
			order = append(order, id)
			return nil
		})
	}

	chain := Chain(mk(1), mk(2), mk(3))
	_ = chain.Handle(context.Background(), types.VerifyRequest{})

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected [1 2 3], got %v", order)
	}
}
