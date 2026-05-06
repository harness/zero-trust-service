package account

import (
	"context"
	"strings"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
)

func TestAllowlist_Valid(t *testing.T) {
	v, err := New(Config{AllowedAccounts: []string{"acc1", "acc2"}})
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
	v, err := New(Config{AllowedAccounts: []string{"acc1"}})
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
	v, err := New(Config{AllowedAccounts: []string{"acc1"}})
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
	v, err := New(Config{AllowedAccounts: []string{"top-level-acc"}})
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
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}
