package validators

import (
	"testing"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func TestBuild_KnownType(t *testing.T) {
	def := ValidatorDef{
		Type: "require_account",
		Config: map[string]any{
			"allowed_accounts": []any{"acc1"},
		},
	}
	v, err := Build(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
}

func TestBuild_UnknownType(t *testing.T) {
	def := ValidatorDef{Type: "nonexistent_validator"}
	_, err := Build(def)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestRegister_CustomFactory(t *testing.T) {
	Register("test_custom", func(cfg map[string]any) (verifier.Interface, error) {
		return verifier.From(nil), nil
	})

	if _, ok := registry["test_custom"]; !ok {
		t.Fatal("expected test_custom in registry")
	}

	// Cleanup
	delete(registry, "test_custom")
}

func TestInit_RegistersOOTBValidators(t *testing.T) {
	expected := []string{"require_account", "shellscript", "image_allowlist", "webhook"}
	for _, name := range expected {
		if _, ok := registry[name]; !ok {
			t.Errorf("expected %q in registry", name)
		}
	}
}
