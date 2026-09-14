package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

const defaultConfigPath = "config.yaml"

// Config holds all runtime configuration.
type Config struct {
	HTTPAddr string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// PersistWorkers controls how many goroutines drain the async persist queue.
	PersistWorkers int

	AuthEnabled bool
	AdminToken  string
	WriterToken string
}

type rawConfig struct {
	HTTPAddr *string `yaml:"httpAddr"`
	MySQL    struct {
		DSN *string `yaml:"dsn"`
	} `yaml:"mysql"`
	Redis struct {
		Addr     *string `yaml:"addr"`
		Password *string `yaml:"password"`
		DB       *int    `yaml:"db"`
	} `yaml:"redis"`
	PersistWorkers *int `yaml:"persistWorkers"`
	Auth           struct {
		Enabled     *bool   `yaml:"enabled"`
		AdminToken  *string `yaml:"adminToken"`
		WriterToken *string `yaml:"writerToken"`
	} `yaml:"auth"`
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	configPath := os.Getenv("RANKFLOW_CONFIG_FILE")
	if configPath == "" {
		configPath = defaultConfigPath
	}
	if err := applyFile(cfg, configPath); err != nil {
		if configPath != defaultConfigPath || !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if err := validateAuthEnabledEnv(); err != nil {
		return nil, err
	}
	envOverrides().apply(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadFromFile(path string) (*Config, error) {
	cfg := defaultConfig()
	if err := applyFile(cfg, path); err != nil {
		return nil, err
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		HTTPAddr:       ":8080",
		MySQLDSN:       "rankflow:rankflow@tcp(127.0.0.1:3306)/rankflow?charset=utf8mb4&parseTime=true&loc=Local",
		RedisAddr:      "127.0.0.1:6379",
		RedisPassword:  "",
		RedisDB:        0,
		PersistWorkers: 2,
		AuthEnabled:    false,
		AdminToken:     "",
		WriterToken:    "",
	}
}

func applyFile(cfg *Config, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw rawConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return err
	}
	raw.apply(cfg)
	return nil
}

func envOverrides() rawConfig {
	var raw rawConfig

	if v, ok := lookupNonEmptyEnv("RANKFLOW_HTTP_ADDR"); ok {
		raw.HTTPAddr = &v
	}
	if v, ok := lookupNonEmptyEnv("RANKFLOW_MYSQL_DSN"); ok {
		raw.MySQL.DSN = &v
	}
	if v, ok := lookupNonEmptyEnv("RANKFLOW_REDIS_ADDR"); ok {
		raw.Redis.Addr = &v
	}
	if v, ok := os.LookupEnv("RANKFLOW_REDIS_PASSWORD"); ok {
		raw.Redis.Password = &v
	}
	if v, ok := lookupEnvInt("RANKFLOW_REDIS_DB"); ok {
		raw.Redis.DB = &v
	}
	if v, ok := lookupEnvInt("RANKFLOW_PERSIST_WORKERS"); ok {
		raw.PersistWorkers = &v
	}
	if v, ok := lookupEnvBool("RANKFLOW_AUTH_ENABLED"); ok {
		raw.Auth.Enabled = &v
	}
	if v, ok := os.LookupEnv("RANKFLOW_ADMIN_TOKEN"); ok {
		raw.Auth.AdminToken = &v
	}
	if v, ok := os.LookupEnv("RANKFLOW_WRITER_TOKEN"); ok {
		raw.Auth.WriterToken = &v
	}

	return raw
}

func (r rawConfig) apply(cfg *Config) {
	if r.HTTPAddr != nil {
		cfg.HTTPAddr = *r.HTTPAddr
	}
	if r.MySQL.DSN != nil {
		cfg.MySQLDSN = *r.MySQL.DSN
	}
	if r.Redis.Addr != nil {
		cfg.RedisAddr = *r.Redis.Addr
	}
	if r.Redis.Password != nil {
		cfg.RedisPassword = *r.Redis.Password
	}
	if r.Redis.DB != nil {
		cfg.RedisDB = *r.Redis.DB
	}
	if r.PersistWorkers != nil {
		cfg.PersistWorkers = *r.PersistWorkers
	}
	if r.Auth.Enabled != nil {
		cfg.AuthEnabled = *r.Auth.Enabled
	}
	if r.Auth.AdminToken != nil {
		cfg.AdminToken = *r.Auth.AdminToken
	}
	if r.Auth.WriterToken != nil {
		cfg.WriterToken = *r.Auth.WriterToken
	}
}

func validate(cfg *Config) error {
	if !cfg.AuthEnabled {
		return nil
	}
	cfg.AdminToken = strings.TrimSpace(cfg.AdminToken)
	cfg.WriterToken = strings.TrimSpace(cfg.WriterToken)
	if cfg.AdminToken == "" {
		return fmt.Errorf("RANKFLOW_ADMIN_TOKEN is required when auth is enabled")
	}
	if cfg.WriterToken == "" {
		return fmt.Errorf("RANKFLOW_WRITER_TOKEN is required when auth is enabled")
	}
	if cfg.AdminToken == cfg.WriterToken {
		return fmt.Errorf("admin and writer tokens must be different")
	}
	return nil
}

func validateAuthEnabledEnv() error {
	v, ok := os.LookupEnv("RANKFLOW_AUTH_ENABLED")
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	if _, err := strconv.ParseBool(v); err != nil {
		return fmt.Errorf("invalid RANKFLOW_AUTH_ENABLED %q: %w", v, err)
	}
	return nil
}

func lookupNonEmptyEnv(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	return v, ok && v != ""
}

func lookupEnvInt(key string) (int, bool) {
	v, ok := lookupNonEmptyEnv(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func lookupEnvBool(key string) (bool, bool) {
	v, ok := lookupNonEmptyEnv(key)
	if !ok {
		return false, false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, false
	}
	return b, true
}
