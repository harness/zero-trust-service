package config

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators"
	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// Config is the top-level ZTS configuration for the example executable.
type Config struct {
	Port       int                      `yaml:"port"`
	AdminPort  int                      `yaml:"admin_port"`
	Validators validators.ValidatorsConfig `yaml:"validators"`
	Audit      AuditConfig              `yaml:"audit"`
	Resolver   resolver.ResolverConfig  `yaml:"resolver"`
}

// AuditConfig holds the YAML-level audit settings.
type AuditConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Dir        string `yaml:"dir"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

// Load reads and parses a YAML config file with ${VAR:-default} env interpolation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	resolved := resolveEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(resolved), &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Port == 0 {
		cfg.Port = 4210
	}
	if cfg.AdminPort == 0 {
		cfg.AdminPort = 8898
	}
	if cfg.Audit.Dir == "" {
		cfg.Audit.Dir = "/var/log/zts/audits"
	}
	if cfg.Audit.MaxAgeDays <= 0 {
		cfg.Audit.MaxAgeDays = 30
	}

	return &cfg, nil
}

// LoadTemplateMappings reads a template mappings YAML file and returns the parsed mappings.
// Returns nil (no error) if the file path is empty or the file does not exist.
func LoadTemplateMappings(filePath string) (map[string]resolver.TemplateMappingConfig, error) {
	if filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("template mappings file %s not found, skipping", filePath)
			return nil, nil
		}
		return nil, fmt.Errorf("read template mappings file %s: %w", filePath, err)
	}

	resolved := resolveEnvVars(string(data))

	var mappings map[string]resolver.TemplateMappingConfig
	if err := yaml.Unmarshal([]byte(resolved), &mappings); err != nil {
		return nil, fmt.Errorf("parse template mappings file %s: %w", filePath, err)
	}

	log.Printf("loaded %d template mapping(s) from %s", len(mappings), filePath)
	return mappings, nil
}

func resolveEnvVars(raw string) string {
	return envVarPattern.ReplaceAllStringFunc(raw, func(match string) string {
		groups := envVarPattern.FindStringSubmatch(match)

		var expr string
		if groups[1] != "" {
			expr = groups[1]
		} else {
			expr = groups[2]
		}

		name := expr
		defaultVal := ""
		if idx := len(name); idx > 0 {
			if i := findDefaultSeparator(expr); i >= 0 {
				name = expr[:i]
				defaultVal = expr[i+2:]
			}
		}

		if v := os.Getenv(name); v != "" {
			return v
		}
		return defaultVal
	})
}

func findDefaultSeparator(s string) int {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == ':' && s[i+1] == '-' {
			return i
		}
	}
	return -1
}
