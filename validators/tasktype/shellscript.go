package tasktype

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// supportedScriptTypes maps Harness script type names to shell parser language
// variants. Only types listed here can be configured and parsed.
var supportedScriptTypes = map[string]syntax.LangVariant{
	"bash": syntax.LangBash,
}
var supportedScriptTypeNames = func() []string {
	var names []string
	for k := range supportedScriptTypes {
		names = append(names, k)
	}
	return names
}()

// shellscript validates shell scripts by ensuring only allowed script types
// and commands are used. The script is parsed into an AST and every command
// invocation is checked against the per-type allowlist.
//
// Config keys are script type names, values are the allowed commands for that type:
//
//	type: shellscript
//	config:
//	  bash:
//	    - "echo"
//	    - "curl"
type shellscript struct {
	commands map[string][]string // script type -> allowed commands
}

// ShellScript creates a shellscript validator from config.
// Each config key must be a supported script type (currently: bash),
// and its value is the list of allowed commands for that type.
func ShellScript(cfg map[string]any) (verifier.Interface, error) {
	if len(cfg) == 0 {
		return nil, fmt.Errorf("shellscript: config must contain at least one script type")
	}

	commands := make(map[string][]string, len(cfg))
	for cfgKey := range cfg {
		scriptType := strings.ToLower(cfgKey)

		if _, ok := supportedScriptTypes[scriptType]; !ok {
			return nil, fmt.Errorf("shellscript: unsupported script type %q (supported: %v)",
				scriptType, supportedScriptTypeNames)
		}

		cmds, err := extractStrings(cfg, cfgKey)
		if err != nil {
			return nil, err
		}

		commands[scriptType] = cmds
	}

	return &shellscript{commands: commands}, nil
}

// scriptParam captures the relevant fields from a shell script task parameter.
type scriptParam struct {
	ScriptType string `json:"scriptType"`
	Script     string `json:"script"`
}

// Handle validates that each parameter uses a configured script type and that
// the script only invokes commands allowed for that type.
func (v *shellscript) Handle(_ context.Context, request types.VerifyRequest) error {
	if request.TaskPackage == nil || request.TaskPackage.TaskDetails == nil || len(request.TaskPackage.TaskDetails.Parameters) == 0 {
		return nil
	}

	var params []scriptParam
	if err := json.Unmarshal(request.TaskPackage.TaskDetails.Parameters, &params); err != nil {
		return fmt.Errorf("shellscript: failed to unmarshal parameters: %w", err)
	}

	for _, p := range params {
		if p.ScriptType == "" {
			continue
		}

		scriptType := strings.ToLower(p.ScriptType)

		allowed, ok := v.commands[scriptType]
		if !ok {
			return fmt.Errorf("shellscript: script type %q is not allowed", scriptType)
		}

		if p.Script == "" {
			continue
		}

		lang := supportedScriptTypes[scriptType]
		if err := validateCommands(p.Script, lang, allowed); err != nil {
			return err
		}
	}

	return nil
}

// validateCommands parses a shell script and walks the AST to ensure every
// command invocation uses a statically-known name from the allowed list.
func validateCommands(script string, lang syntax.LangVariant, allowed []string) error {
	parser := syntax.NewParser(syntax.Variant(lang))
	f, err := parser.Parse(strings.NewReader(script), "")
	if err != nil {
		return fmt.Errorf("shellscript: failed to parse script: %w", err)
	}

	var walkErr error
	syntax.Walk(f, func(node syntax.Node) bool {
		if node == nil || walkErr != nil {
			return false
		}

		switch n := node.(type) {
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				return true // pure assignment, e.g. FOO=bar
			}

			name, ok := staticCommandName(n.Args[0])
			if !ok {
				walkErr = fmt.Errorf("shellscript: dynamic command name is not allowed")
				return false
			}

			if !slices.Contains(allowed, name) {
				walkErr = fmt.Errorf("shellscript: command %q is not allowed", name)
				return false
			}

		case *syntax.DeclClause:
			// Covers: export, declare, local, readonly, typeset, nameref
			name := n.Variant.Value
			if !slices.Contains(allowed, name) {
				walkErr = fmt.Errorf("shellscript: command %q is not allowed", name)
				return false
			}
		}

		return true
	})

	return walkErr
}

// staticCommandName extracts the command name from a word only when all parts
// are literal (no variable expansions, command substitutions, etc.).
func staticCommandName(word *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range word.Parts {
		lit, ok := part.(*syntax.Lit)
		if !ok {
			return "", false
		}
		b.WriteString(lit.Value)
	}
	return b.String(), b.Len() > 0
}

func extractStrings(cfg map[string]any, key string) ([]string, error) {
	raw, ok := cfg[key]
	if !ok {
		return nil, fmt.Errorf("shellscript: missing '%s' in config", key)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("shellscript: '%s' must be a list", key)
	}

	var result []string
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("shellscript: each %s must be a string, got %T", key, v)
		}
		result = append(result, s)
	}
	return result, nil
}
