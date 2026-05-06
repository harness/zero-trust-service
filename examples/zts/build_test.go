package main

import (
	"context"
	"fmt"
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/examples/zts/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/account"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/tasktype"
	"gopkg.in/yaml.v3"
)

func yamlNode(t *testing.T, src string) yaml.Node {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(src), &node); err != nil {
		t.Fatalf("bad yaml: %v", err)
	}
	return *node.Content[0]
}

type factory func(cfg any) (verifier.Interface, error)

type testRegistry struct {
	factories map[string]factory
}

func newTestRegistry() *testRegistry {
	return &testRegistry{factories: make(map[string]factory)}
}

func (r *testRegistry) register(name string, f factory) { r.factories[name] = f }

func (r *testRegistry) resolve(name string, cfg any) (verifier.Interface, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown verifier type: %q", name)
	}
	return f(cfg)
}

func yamlFactory[C any](fn func(C) (verifier.Interface, error)) factory {
	return func(cfg any) (verifier.Interface, error) {
		node, ok := cfg.(yaml.Node)
		if !ok {
			return nil, fmt.Errorf("expected yaml.Node, got %T", cfg)
		}
		var typed C
		if err := node.Decode(&typed); err != nil {
			return nil, err
		}
		return fn(typed)
	}
}

func newTestResolve() ResolveFunc {
	reg := newTestRegistry()
	reg.register("require_account", yamlFactory(account.New))
	reg.register("shellscript", yamlFactory(tasktype.NewShellScript))
	reg.register("image_allowlist", yamlFactory(tasktype.NewImageAllowlist))
	return reg.resolve
}

func TestBuildChain_GlobalOnly(t *testing.T) {
	m := metrics.NewNoop()
	resolve := newTestResolve()
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
			{
				Type:   "require_account",
				Config: yamlNode(t, `allowed_accounts: ["acc1"]`),
			},
		},
	}

	chain, err := BuildChain(cfg, m, resolve, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc1"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	req2 := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "acc2"}}}
	if err := chain.Handle(context.Background(), req2); err == nil {
		t.Fatal("expected error for blocked account")
	}
}

func TestBuildChain_WithTaskType(t *testing.T) {
	m := metrics.NewNoop()
	resolve := newTestResolve()
	cfg := config.ValidatorsConfig{
		ByTaskType: map[string][]config.ValidatorDef{
			"SHELL_SCRIPT_TASK_NG": {
				{
					Type:   "shellscript",
					Config: yamlNode(t, `bash: ["rm"]`),
				},
			},
		},
	}

	chain, err := BuildChain(cfg, m, resolve, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{
		TaskPackage: &types.DelegateTaskPackage{
			TaskDetails: &types.TaskDetails{TaskType: "OTHER"},
		},
	}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass for non-matching task type, got %v", err)
	}
}

func TestBuildChain_Empty(t *testing.T) {
	m := metrics.NewNoop()
	resolve := newTestResolve()
	cfg := config.ValidatorsConfig{}

	chain, err := BuildChain(cfg, m, resolve, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := chain.Handle(context.Background(), types.VerifyRequest{}); err != nil {
		t.Fatalf("expected pass for empty config, got %v", err)
	}
}

func TestBuildChain_DisabledValidator(t *testing.T) {
	m := metrics.NewNoop()
	resolve := newTestResolve()
	disabled := false
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
			{
				Type:    "require_account",
				Enabled: &disabled,
				Config:  yamlNode(t, `allowed_accounts: ["acc1"]`),
			},
		},
	}

	chain, err := BuildChain(cfg, m, resolve, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := types.VerifyRequest{TaskPackage: &types.DelegateTaskPackage{ZTSMetadata: &types.ZTSMetadata{AccountID: "unknown"}}}
	if err := chain.Handle(context.Background(), req); err != nil {
		t.Fatalf("expected pass with disabled validator, got %v", err)
	}
}

func TestBuildChain_UnknownValidatorType(t *testing.T) {
	m := metrics.NewNoop()
	resolve := newTestResolve()
	cfg := config.ValidatorsConfig{
		Global: []config.ValidatorDef{
			{Type: "nonexistent"},
		},
	}

	_, err := BuildChain(cfg, m, resolve, nil)
	if err == nil {
		t.Fatal("expected error for unknown validator type")
	}
}
