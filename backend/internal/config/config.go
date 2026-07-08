package config

import (
	"errors"
	"os"
	"strconv"

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
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	configPath := os.Getenv("RANKFLOW_CONFIG_FILE")
	if configPath == "" {
		configPath = defaultConfigPath
	}
	if err := applyFile(cfg, configPath); err != nil {
		if configPath != defaultConfigPath || !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
	}

	envOverrides().apply(cfg)
	return cfg, nil
}

func LoadFromFile(path string) (*Config, error) {
	cfg := defaultConfig()
	if err := applyFile(cfg, path); err != nil {
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
