package account

import (
	"context"
	"strings"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestAllowlist_Valid(t *testing.T) {
	v, err := Allowlist(map[string]any{
		"allowed_accounts": []any{"acc1", "acc2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"},
		},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestAllowlist_Blocked(t *testing.T) {
	v, err := Allowlist(map[string]any{
		"allowed_accounts": []any{"acc1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			ZTSMetadata: &types.ZTSMetadata{AccountID: "acc999"},
		},
	}
	if err := v.Handle(context.Background(), req); err == nil {
		t.Fatal("expected error for blocked account")
	}
}

func TestAllowlist_MissingAccountID(t *testing.T) {
	v, err := Allowlist(map[string]any{
		"allowed_accounts": []any{"acc1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{}
	err = v.Handle(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected 'missing' in error, got %v", err)
	}
}

func TestAllowlist_FallbackToTopLevelAccountID(t *testing.T) {
	v, err := Allowlist(map[string]any{
		"allowed_accounts": []any{"top-level-acc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{AccountID: "top-level-acc"},
	}
	if err := v.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass with top-level account, got %v", err)
	}
}

func TestAllowlist_ConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
	}{
		{"missing key", map[string]any{}},
		{"not a list", map[string]any{"allowed_accounts": "not-a-list"}},
		{"empty list", map[string]any{"allowed_accounts": []any{}}},
		{"non-string item", map[string]any{"allowed_accounts": []any{123}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Allowlist(tc.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
