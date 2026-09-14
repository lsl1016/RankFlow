package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAuthFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`auth:
  enabled: true
  adminToken: "admin-secret"
  writerToken: "writer-secret"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load auth config: %v", err)
	}
	if !cfg.AuthEnabled || cfg.AdminToken != "admin-secret" || cfg.WriterToken != "writer-secret" {
		t.Fatalf("unexpected auth config: %+v", cfg)
	}
}

func TestAuthEnabledRequiresBothDistinctTokens(t *testing.T) {
	tests := []struct {
		name   string
		admin  string
		writer string
	}{
		{name: "missing admin", writer: "writer-secret"},
		{name: "missing writer", admin: "admin-secret"},
		{name: "same token", admin: "same-secret", writer: "same-secret"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.AuthEnabled = true
			cfg.AdminToken = tc.admin
			cfg.WriterToken = tc.writer
			if err := validate(cfg); err == nil {
				t.Fatalf("expected invalid auth config: %+v", cfg)
			}
		})
	}
}

func TestAuthEnvironmentOverrides(t *testing.T) {
	t.Setenv("RANKFLOW_AUTH_ENABLED", "true")
	t.Setenv("RANKFLOW_ADMIN_TOKEN", "env-admin")
	t.Setenv("RANKFLOW_WRITER_TOKEN", "env-writer")

	cfg := defaultConfig()
	envOverrides().apply(cfg)
	if err := validate(cfg); err != nil {
		t.Fatalf("validate auth env: %v", err)
	}
	if !cfg.AuthEnabled || cfg.AdminToken != "env-admin" || cfg.WriterToken != "env-writer" {
		t.Fatalf("unexpected auth env config: %+v", cfg)
	}
}

func TestMalformedAuthEnabledEnvironmentIsRejected(t *testing.T) {
	t.Setenv("RANKFLOW_AUTH_ENABLED", "tru")
	if err := validateAuthEnabledEnv(); err == nil {
		t.Fatal("expected malformed RANKFLOW_AUTH_ENABLED to be rejected")
	}
}
