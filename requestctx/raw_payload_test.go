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

package requestctx

import (
	"context"
	"testing"
)

func TestWithRawPayload_RoundTrip(t *testing.T) {
	body := []byte(`{"taskId":"abc"}`)
	ctx := WithRawPayload(context.Background(), body)
	got := RawPayloadFrom(ctx)
	if string(got) != string(body) {
		t.Errorf("RawPayloadFrom = %q, want %q", got, body)
	}
}

func TestRawPayloadFrom_Missing(t *testing.T) {
	got := RawPayloadFrom(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %q", got)
	}
}

func TestWithRawPayload_Nil(t *testing.T) {
	ctx := WithRawPayload(context.Background(), nil)
	got := RawPayloadFrom(ctx)
	if got != nil {
		t.Errorf("expected nil for nil payload, got %q", got)
	}
}
