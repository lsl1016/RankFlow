package config

import (
	"os"
	"path/filepath"
<<<<<<< HEAD
	"strings"
	"testing"
)

func TestLoadFileLoadsTOML(t *testing.T) {
	path := writeTempTOML(t, `
[server]
http_addr = ":9090"

[mysql]
dsn = "remote:secret@tcp(10.0.0.10:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local"

[redis]
addr = "10.0.0.11:6379"
password = "redis-secret"
db = 3

[persist]
workers = 8
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.MySQLDSN != "remote:secret@tcp(10.0.0.10:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local" {
		t.Fatalf("MySQLDSN = %q", cfg.MySQLDSN)
	}
	if cfg.RedisAddr != "10.0.0.11:6379" {
		t.Fatalf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "redis-secret" {
		t.Fatalf("RedisPassword = %q", cfg.RedisPassword)
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("RedisDB = %d", cfg.RedisDB)
	}
	if cfg.PersistWorkers != 8 {
		t.Fatalf("PersistWorkers = %d", cfg.PersistWorkers)
	}
}

func TestLoadFileRequiresTOMLFields(t *testing.T) {
	path := writeTempTOML(t, `
[server]
http_addr = ":9090"
`)

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, field := range []string{"mysql.dsn", "redis.addr", "persist.workers"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("error %q does not mention %s", err.Error(), field)
		}
	}
}

func TestLoadFileRejectsInvalidTOML(t *testing.T) {
	path := writeTempTOML(t, `
[server]
http_addr = [
`)

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse config file") {
		t.Fatalf("error = %q", err.Error())
	}
}

func writeTempTOML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
=======
	"testing"
)

func TestLoadFromYAMLFile(t *testing.T) {
	t.Setenv("RANKFLOW_HTTP_ADDR", "")
	t.Setenv("RANKFLOW_MYSQL_DSN", "")
	t.Setenv("RANKFLOW_REDIS_ADDR", "")
	t.Setenv("RANKFLOW_REDIS_PASSWORD", "")
	t.Setenv("RANKFLOW_REDIS_DB", "")
	t.Setenv("RANKFLOW_PERSIST_WORKERS", "")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`httpAddr: ":9090"
mysql:
  dsn: "user:pass@tcp(10.0.0.10:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local"
redis:
  addr: "10.0.0.20:6379"
  password: "secret"
  db: 3
persistWorkers: 5
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":9090")
	}
	if cfg.MySQLDSN != "user:pass@tcp(10.0.0.10:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local" {
		t.Fatalf("MySQLDSN = %q", cfg.MySQLDSN)
	}
	if cfg.RedisAddr != "10.0.0.20:6379" {
		t.Fatalf("RedisAddr = %q, want %q", cfg.RedisAddr, "10.0.0.20:6379")
	}
	if cfg.RedisPassword != "secret" {
		t.Fatalf("RedisPassword = %q, want %q", cfg.RedisPassword, "secret")
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("RedisDB = %d, want %d", cfg.RedisDB, 3)
	}
	if cfg.PersistWorkers != 5 {
		t.Fatalf("PersistWorkers = %d, want %d", cfg.PersistWorkers, 5)
	}
}

func TestLoadWithoutYAMLUsesEnvAndDefaults(t *testing.T) {
	t.Setenv("RANKFLOW_HTTP_ADDR", ":8088")
	t.Setenv("RANKFLOW_MYSQL_DSN", "env-user:env-pass@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local")
	t.Setenv("RANKFLOW_REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("RANKFLOW_REDIS_PASSWORD", "env-secret")
	t.Setenv("RANKFLOW_REDIS_DB", "9")
	t.Setenv("RANKFLOW_PERSIST_WORKERS", "7")

	cfg := Load()

	if cfg.HTTPAddr != ":8088" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8088")
	}
	if cfg.MySQLDSN != "env-user:env-pass@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local" {
		t.Fatalf("MySQLDSN = %q", cfg.MySQLDSN)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" {
		t.Fatalf("RedisAddr = %q, want %q", cfg.RedisAddr, "127.0.0.1:6380")
	}
	if cfg.RedisPassword != "env-secret" {
		t.Fatalf("RedisPassword = %q, want %q", cfg.RedisPassword, "env-secret")
	}
	if cfg.RedisDB != 9 {
		t.Fatalf("RedisDB = %d, want %d", cfg.RedisDB, 9)
	}
	if cfg.PersistWorkers != 7 {
		t.Fatalf("PersistWorkers = %d, want %d", cfg.PersistWorkers, 7)
	}
}

func TestLoadEnvOverridesYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`httpAddr: ":9090"
mysql:
  dsn: "yaml-user:yaml-pass@tcp(10.0.0.10:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local"
redis:
  addr: "10.0.0.20:6379"
  password: "yaml-secret"
  db: 3
persistWorkers: 5
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("RANKFLOW_CONFIG_FILE", configPath)
	t.Setenv("RANKFLOW_HTTP_ADDR", ":8088")
	t.Setenv("RANKFLOW_MYSQL_DSN", "env-user:env-pass@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local")
	t.Setenv("RANKFLOW_REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("RANKFLOW_REDIS_PASSWORD", "env-secret")
	t.Setenv("RANKFLOW_REDIS_DB", "9")
	t.Setenv("RANKFLOW_PERSIST_WORKERS", "7")

	cfg := Load()

	if cfg.HTTPAddr != ":8088" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8088")
	}
	if cfg.MySQLDSN != "env-user:env-pass@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local" {
		t.Fatalf("MySQLDSN = %q", cfg.MySQLDSN)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" {
		t.Fatalf("RedisAddr = %q, want %q", cfg.RedisAddr, "127.0.0.1:6380")
	}
	if cfg.RedisPassword != "env-secret" {
		t.Fatalf("RedisPassword = %q, want %q", cfg.RedisPassword, "env-secret")
	}
	if cfg.RedisDB != 9 {
		t.Fatalf("RedisDB = %d, want %d", cfg.RedisDB, 9)
	}
	if cfg.PersistWorkers != 7 {
		t.Fatalf("PersistWorkers = %d, want %d", cfg.PersistWorkers, 7)
	}
}
>>>>>>> 8a2d5097677a99bc1cf2fe378f95e0b18cb8d416
