package config

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigFile = "conf/app.toml"

// Config holds all runtime configuration loaded from a TOML file.
type Config struct {
	HTTPAddr string

	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// PersistWorkers controls how many goroutines drain the async persist queue.
	PersistWorkers int
}

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
		return nil, err
	}
	return cfg, nil
}

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
}
