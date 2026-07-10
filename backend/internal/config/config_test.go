package config

import (
	"os"
	"path/filepath"
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
