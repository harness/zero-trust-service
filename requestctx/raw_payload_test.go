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
