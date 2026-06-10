package tasktype

import (
	"context"
	"encoding/json"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestImageAllowlist_Allowed(t *testing.T) {
	v, err := NewImageAllowlist(ImageAllowlistConfig{AllowedPrefixes: []string{"harness/", "library/"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := json.RawMessage(`[{"imageDetails":{"name":"harness/ci-lite-engine:latest"}}]`)
	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "INITIALIZATION_PHASE",
				Parameters: params,
			},
		},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestImageAllowlist_Blocked(t *testing.T) {
	v, err := NewImageAllowlist(ImageAllowlistConfig{AllowedPrefixes: []string{"harness/"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := json.RawMessage(`[{"imageDetails":{"name":"malicious/miner:latest"}}]`)
	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
			TaskDetails: &types.TaskDetails{
				TaskType:   "INITIALIZATION_PHASE",
				Parameters: params,
			},
		},
	}
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected blocked")
	}
}

func TestImageAllowlist_NilTaskDetails(t *testing.T) {
	v, err := NewImageAllowlist(ImageAllowlistConfig{AllowedPrefixes: []string{"harness/"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := v.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for nil task details, got %v", err)
	}
}

func TestImageAllowlist_ConfigErrors(t *testing.T) {
	_, err := NewImageAllowlist(ImageAllowlistConfig{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestExtractImageNames(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		expect int
	}{
		{"single", `[{"imageDetails":{"name":"img1"}}]`, 1},
		{"nested", `[{"a":{"imageDetails":{"name":"img2"}}}]`, 1},
		{"array", `[{"imageDetails":{"name":"img3"}},{"imageDetails":{"name":"img4"}}]`, 2},
		{"no images", `[{"command":"echo"}]`, 0},
		{"empty name", `[{"imageDetails":{"name":""}}]`, 0},
		{"invalid json", `not-json`, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := extractImageNames(json.RawMessage(tc.json))
			if len(names) != tc.expect {
				t.Fatalf("expected %d images, got %d: %v", tc.expect, len(names), names)
			}
		})
	}
}
