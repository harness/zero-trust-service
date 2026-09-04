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

package main

import (
	"context"
	"testing"

	"github.com/harness/zero-trust-service/types"
	"github.com/harness/zero-trust-service/verifier"
	"gopkg.in/yaml.v3"
)

func nopVerifier() verifier.Interface {
	return verifier.From(func(_ context.Context, _ types.VerifyRequest) error { return nil })
}

func TestRegistry_RegisterAndResolve(t *testing.T) {
	reg := NewRegistry()
	reg.Register("nop", func(_ any) (verifier.Interface, error) {
		return nopVerifier(), nil
	})
	v, err := reg.Resolve("nop", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
}

func TestRegistry_UnknownType(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Resolve("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown verifier type")
	}
}

func TestRegistryTyped_DecodesYAML(t *testing.T) {
	type cfg struct {
		Value string `yaml:"value"`
	}
	reg := NewRegistry()
	var got cfg
	RegisterTyped(reg, "typed", func(c cfg) (verifier.Interface, error) {
		got = c
		return nopVerifier(), nil
	})

	var node yaml.Node
	if err := yaml.Unmarshal([]byte("value: hello"), &node); err != nil {
		t.Fatal(err)
	}
	_, err := reg.Resolve("typed", *node.Content[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Value != "hello" {
		t.Errorf("expected value=hello, got %q", got.Value)
	}
}

func TestRegistryTyped_WrongConfigType(t *testing.T) {
	type cfg struct{ Value string }
	reg := NewRegistry()
	RegisterTyped(reg, "typed", func(c cfg) (verifier.Interface, error) {
		return nopVerifier(), nil
	})
	_, err := reg.Resolve("typed", "not-a-yaml-node")
	if err == nil {
		t.Fatal("expected error for wrong config type")
	}
}

func TestDefaultRegistry_HasBuiltins(t *testing.T) {
	reg := DefaultRegistry()
	for _, name := range []string{
		"require_account", "task_denylist", "shellscript", "image_allowlist", "step_lookup", "webhook",
	} {
		if _, ok := reg.factories[name]; !ok {
			t.Errorf("DefaultRegistry missing builtin %q", name)
		}
	}
}
