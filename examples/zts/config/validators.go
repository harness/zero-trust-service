package config

import "gopkg.in/yaml.v3"

// ValidatorsConfig defines which validators to run.
type ValidatorsConfig struct {
	Global     []ValidatorDef            `yaml:"global"`
	ByTaskType map[string][]ValidatorDef `yaml:"by_task_type"`
	Custom     []ValidatorDef            `yaml:"custom"`
}

// ValidatorDef describes a single validator instance.
type ValidatorDef struct {
	Type    string    `yaml:"type"`
	Enabled *bool     `yaml:"enabled,omitempty"`
	Config  yaml.Node `yaml:"config"`
}

// IsEnabled returns true if the validator is enabled (default: true when omitted).
func (v ValidatorDef) IsEnabled() bool {
	return v.Enabled == nil || *v.Enabled
}
