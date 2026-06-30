package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration, loaded from environment variables
// with sensible defaults for local docker-compose development.
type Config struct {
	HTTPAddr string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// PersistWorkers controls how many goroutines drain the async persist queue.
	PersistWorkers int
}

func Load() *Config {
	return &Config{
		HTTPAddr:       env("RANKFLOW_HTTP_ADDR", ":8080"),
		MySQLDSN:       env("RANKFLOW_MYSQL_DSN", "rankflow:rankflow@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local"),
		RedisAddr:      env("RANKFLOW_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:  env("RANKFLOW_REDIS_PASSWORD", ""),
		RedisDB:        envInt("RANKFLOW_REDIS_DB", 0),
		PersistWorkers: envInt("RANKFLOW_PERSIST_WORKERS", 2),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
