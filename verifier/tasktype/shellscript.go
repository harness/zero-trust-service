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

// ShellScriptConfig holds the shell script validator configuration.
// Keys are script type names, values are allowed commands for that type.
type ShellScriptConfig struct {
	Bash []string `yaml:"bash"`
}

type shellScriptVerifier struct {
	bash []string
}

// NewShellScript creates a shellscript validator from typed config.
func NewShellScript(cfg ShellScriptConfig) (verifier.Interface, error) {
	if len(cfg.Bash) == 0 {
		return nil, fmt.Errorf("shellscript: config must contain at least one script type")
	}
	return &shellScriptVerifier{bash: cfg.Bash}, nil
}

type scriptParam struct {
	ScriptType string `json:"scriptType"`
	Script     string `json:"script"`
}

func (v *shellScriptVerifier) allowedCommands(scriptType string) ([]string, bool) {
	switch scriptType {
	case "bash":
		if len(v.bash) > 0 {
			return v.bash, true
		}
	}
	return nil, false
}

func (v *shellScriptVerifier) Handle(_ context.Context, request types.VerifyRequest) error {
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

		allowed, ok := v.allowedCommands(scriptType)
		if !ok {
			return fmt.Errorf("shellscript: script type %q is not allowed (supported: %v)", scriptType, supportedScriptTypeNames)
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
				return true
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
