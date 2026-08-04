package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

func TestLoad_Values(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantPort     int
		wantAdmin    int
		wantDir      string
		wantMaxAge   int
	}{
		{"defaults", "{}\n", 4210, 8898, "/var/log/zts/audits", 30},
		{"explicit values", "port: 9090\nadmin_port: 9091\naudit:\n  dir: /tmp/audits\n  max_age_days: 7\n", 9090, 9091, "/tmp/audits", 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeTemp(t, tc.content))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tc.wantPort)
			}
			if cfg.AdminPort != tc.wantAdmin {
				t.Errorf("AdminPort = %d, want %d", cfg.AdminPort, tc.wantAdmin)
			}
			if cfg.Audit.Dir != tc.wantDir {
				t.Errorf("Audit.Dir = %q, want %q", cfg.Audit.Dir, tc.wantDir)
			}
			if cfg.Audit.MaxAgeDays != tc.wantMaxAge {
				t.Errorf("Audit.MaxAgeDays = %d, want %d", cfg.Audit.MaxAgeDays, tc.wantMaxAge)
			}
		})
	}
}

func TestLoad_EnvInterpolation(t *testing.T) {
	t.Setenv("ZTS_PORT_TEST", "7777")
	_ = os.Unsetenv("ZTS_UNSET_VAR")

	// Set var used; unset var falls back to the :- default.
	cfg, err := Load(writeTemp(t, "port: ${ZTS_PORT_TEST:-4210}\nadmin_port: ${ZTS_UNSET_VAR:-5555}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 7777 {
		t.Errorf("Port = %d, want 7777", cfg.Port)
	}
	if cfg.AdminPort != 5555 {
		t.Errorf("AdminPort = %d, want 5555 (default)", cfg.AdminPort)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, ":\t:")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadTemplateMappings_Empty(t *testing.T) {
	m, err := LoadTemplateMappings("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil for empty path, got %v", m)
	}
}

func TestLoadTemplateMappings_NotExist(t *testing.T) {
	m, err := LoadTemplateMappings("/nonexistent/mappings.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestLoadTemplateMappings_Valid(t *testing.T) {
	path := writeTemp(t, `
myTemplate:
  provider: github
  version: v1
`)
	m, err := LoadTemplateMappings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(m))
	}
	if m["myTemplate"].Provider != "github" {
		t.Errorf("Provider = %q, want github", m["myTemplate"].Provider)
	}
}

func TestLoadTemplateMappings_InvalidYAML(t *testing.T) {
	path := writeTemp(t, ":\t:")
	_, err := LoadTemplateMappings(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadTemplateMappings_ReadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mappings.yaml")
	// Create as a directory so ReadFile fails
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTemplateMappings(path)
	if err == nil {
		t.Fatal("expected error reading a directory as file")
	}
}
