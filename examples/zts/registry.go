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
	"fmt"

	"github.com/harness/zero-trust-service/examples/webhook_server/webhook"
	"github.com/harness/zero-trust-service/verifier"
	"github.com/harness/zero-trust-service/examples/zts/ordering"
	"github.com/harness/zero-trust-service/verifier/account"
	"github.com/harness/zero-trust-service/verifier/pipeline"
	"github.com/harness/zero-trust-service/verifier/taskdenylist"
	"github.com/harness/zero-trust-service/verifier/tasktype"
	"gopkg.in/yaml.v3"
)

// Factory creates a verifier from an arbitrary config value.
type Factory func(cfg any) (verifier.Interface, error)

// Registry stores named verifier factories.
type Registry struct {
	factories map[string]Factory
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a verifier factory by name.
func (r *Registry) Register(name string, f Factory) {
	r.factories[name] = f
}

// Resolve creates a verifier from the factory registered under name, using cfg.
func (r *Registry) Resolve(name string, cfg any) (verifier.Interface, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown verifier type: %q", name)
	}
	return f(cfg)
}

// DefaultRegistry returns a Registry pre-populated with all built-in verifier types.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	RegisterTyped(reg, "require_account", account.New)
	RegisterTyped(reg, "task_denylist", taskdenylist.New)
	RegisterTyped(reg, "shellscript", tasktype.NewShellScript)
	RegisterTyped(reg, "image_allowlist", tasktype.NewImageAllowlist)
	RegisterTyped(reg, "step_lookup", pipeline.NewStepLookup)
	RegisterTyped(reg, "execution_ordering", ordering.New)
	RegisterTyped(reg, "webhook", webhook.New)
	return reg
}

// RegisterTyped registers a typed constructor that decodes config from a yaml.Node.
func RegisterTyped[C any](r *Registry, name string, fn func(C) (verifier.Interface, error)) {
	r.Register(name, func(cfg any) (verifier.Interface, error) {
		node, ok := cfg.(yaml.Node)
		if !ok {
			return nil, fmt.Errorf("expected yaml.Node config, got %T", cfg)
		}
		var typed C
		if err := node.Decode(&typed); err != nil {
			return nil, fmt.Errorf("%s: invalid config: %w", name, err)
		}
		return fn(typed)
	})
}
