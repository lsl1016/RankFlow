package config

import (
<<<<<<< HEAD
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigFile = "conf/app.toml"

// Config holds all runtime configuration loaded from a TOML file.
=======
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

const defaultConfigPath = "config.yaml"

// Config holds all runtime configuration.
>>>>>>> 8a2d5097677a99bc1cf2fe378f95e0b18cb8d416
type Config struct {
	HTTPAddr string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// PersistWorkers controls how many goroutines drain the async persist queue.
	PersistWorkers int
}

<<<<<<< HEAD
type fileConfig struct {
	Server struct {
		HTTPAddr string `toml:"http_addr"`
	} `toml:"server"`
	MySQL struct {
		DSN string `toml:"dsn"`
	} `toml:"mysql"`
	Redis struct {
		Addr     string `toml:"addr"`
		Password string `toml:"password"`
		DB       int    `toml:"db"`
	} `toml:"redis"`
	Persist struct {
		Workers int `toml:"workers"`
	} `toml:"persist"`
}

func Load() (*Config, error) {
	return LoadFile(configPath())
}

func LoadFile(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var raw fileConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	cfg := &Config{
		HTTPAddr:       strings.TrimSpace(raw.Server.HTTPAddr),
		MySQLDSN:       strings.TrimSpace(raw.MySQL.DSN),
		RedisAddr:      strings.TrimSpace(raw.Redis.Addr),
		RedisPassword:  raw.Redis.Password,
		RedisDB:        raw.Redis.DB,
		PersistWorkers: raw.Persist.Workers,
	}
	if err := cfg.validate(path); err != nil {
=======
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

func Load() *Config {
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
	return cfg
}

func LoadFromFile(path string) (*Config, error) {
	cfg := defaultConfig()
	if err := applyFile(cfg, path); err != nil {
>>>>>>> 8a2d5097677a99bc1cf2fe378f95e0b18cb8d416
		return nil, err
	}
	return cfg, nil
}

<<<<<<< HEAD
func configPath() string {
	if flag.Lookup("conf") == nil {
		flag.String("conf", defaultConfigFile, "RankFlow TOML config file path")
	}
	if !flag.Parsed() {
		flag.Parse()
	}
	return flag.Lookup("conf").Value.String()
}

func (c *Config) validate(path string) error {
	var missing []string
	if c.HTTPAddr == "" {
		missing = append(missing, "server.http_addr")
	}
	if c.MySQLDSN == "" {
		missing = append(missing, "mysql.dsn")
	}
	if c.RedisAddr == "" {
		missing = append(missing, "redis.addr")
	}
	if c.PersistWorkers <= 0 {
		missing = append(missing, "persist.workers")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config file %q missing required fields: %s", path, strings.Join(missing, ", "))
	}
	if c.RedisDB < 0 {
		return fmt.Errorf("config file %q has invalid redis.db: %d", path, c.RedisDB)
	}
	return nil
=======
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
>>>>>>> 8a2d5097677a99bc1cf2fe378f95e0b18cb8d416
}
