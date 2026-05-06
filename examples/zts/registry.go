package main

import (
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/examples/webhook_server/webhook"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/account"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/pipeline"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/tasktype"
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

// Resolve creates a verifier by looking up the factory for the given name
// and calling it with the provided config.
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
	RegisterTyped(reg, "shellscript", tasktype.NewShellScript)
	RegisterTyped(reg, "image_allowlist", tasktype.NewImageAllowlist)
	RegisterTyped(reg, "step_lookup", pipeline.NewStepLookup)
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
