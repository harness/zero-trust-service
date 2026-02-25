package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level ZTS configuration
type Config struct {
	Port       int              `yaml:"port"`
	Validators ValidatorsConfig `yaml:"validators"`
	Audit      AuditConfig      `yaml:"audit"`
}

// AuditConfig controls local file-based audit logging.
type AuditConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Dir        string `yaml:"dir"`          // base directory for audit files
	MaxAgeDays int    `yaml:"max_age_days"` // auto-delete metadata + payload files older than this (default: 30)
}

// ValidatorsConfig defines which validators to run
type ValidatorsConfig struct {
	// Global validators run for every request (Harness OOTB)
	Global []ValidatorDef `yaml:"global"`
	// ByTaskType maps task_type → list of validators
	ByTaskType map[string][]ValidatorDef `yaml:"by_task_type"`
	// Custom validators — customer-provided, run for every request (e.g. webhooks)
	Custom []ValidatorDef `yaml:"custom"`
}

// ValidatorDef describes a single validator instance
type ValidatorDef struct {
	Type    string         `yaml:"type"`
	Enabled *bool          `yaml:"enabled,omitempty"` // nil or true = enabled, false = disabled
	Config  map[string]any `yaml:"config"`
}

// IsEnabled returns true if the validator is enabled (default: true when omitted).
func (v ValidatorDef) IsEnabled() bool {
	return v.Enabled == nil || *v.Enabled
}

// Load reads a YAML config file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Port == 0 {
		cfg.Port = 4210
	}
	// Audit defaults
	if cfg.Audit.Dir == "" {
		cfg.Audit.Dir = "/var/log/zts/audits"
	}
	if cfg.Audit.MaxAgeDays <= 0 {
		cfg.Audit.MaxAgeDays = 30
	}
	return &cfg, nil
}
