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

func TestToHandler_Authorized(t *testing.T) {
	v := From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
	handler := ToHandler(v)

	resp, err := handler(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed=true, got false")
	}
	if resp.Reason != "" {
		t.Fatalf("expected empty reason, got %q", resp.Reason)
	}
}

func TestToHandler_Unauthorized(t *testing.T) {
	v := From(func(_ context.Context, _ types.VerifyRequest) error {
		return errors.New("not allowed")
	})
	handler := ToHandler(v)

	resp, err := handler(context.Background(), types.VerifyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Fatalf("expected allowed=false, got true")
	}
	if resp.Reason != "not allowed" {
		t.Fatalf("expected reason 'not allowed', got %q", resp.Reason)
	}
}
