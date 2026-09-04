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

package account

import (
	"context"
	"strings"
	"testing"

	"github.com/harness/zero-trust-service/types"
)

func TestAllowlist_Valid(t *testing.T) {
	v, err := New(Config{AllowedAccounts: []string{"acc1", "acc2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.TaskPackage{
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
		TaskPackage: &types.TaskPackage{
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
		TaskPackage: &types.TaskPackage{AccountID: "top-level-acc"},
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
