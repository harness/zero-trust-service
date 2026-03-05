package validators

import (
	"fmt"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/account"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/custom"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/pipeline"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators/tasktype"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// Factory creates a verifier from a config map.
type Factory func(cfg map[string]any) (verifier.Interface, error)

var registry = map[string]Factory{}

func init() {
	// Harness OOTB validators — each maps config "type" → constructor
	Register("require_account", account.Allowlist)
	Register("shellscript", tasktype.ShellScript)
	Register("image_allowlist", tasktype.ImageAllowlist)

	// Pipeline-aware validators (use resolved pipeline from context)
	Register("step_lookup", pipeline.StepLookup)

	// Customer-facing validators
	Register("webhook", custom.Webhook)
}

// Register adds a validator factory.
// Customers call this to add custom validator types.
func Register(name string, factory Factory) {
	registry[name] = factory
}

// Build creates a single verifier from a ValidatorDef.
func Build(def ValidatorDef) (verifier.Interface, error) {
	factory, ok := registry[def.Type]
	if !ok {
		return nil, fmt.Errorf("unknown validator type: %q", def.Type)
	}
	return factory(def.Config)
}
