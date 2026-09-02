package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all local-only configuration for Chirp.
// Priority: environment variable > config.json > default value.
type Config struct {
	Port       string
	SQLitePath string
	JWTSecret  string
	UploadDir  string
}

func Load() *Config {
	filePath := os.Getenv("CONFIG_FILE")
	if filePath == "" {
		filePath = "config.json"
	}

	var fileCfg *Config
	if exists(filePath) {
		cfg, err := loadFromFile(filePath)
		if err == nil {
			fileCfg = cfg
		} else {
			fmt.Fprintf(os.Stderr, "warn: load config file %s failed: %v\n", filePath, err)
		}
	}

	cfg := &Config{}

	// Resolve with priority: env > file > default
	cfg.Port = firstNonEmpty(os.Getenv("PORT"), fileCfgValue(fileCfg, func(c *Config) string { return c.Port }), "9527")
	cfg.SQLitePath = firstNonEmpty(os.Getenv("SQLITE_PATH"), fileCfgValue(fileCfg, func(c *Config) string { return c.SQLitePath }), "chirp.db")
	cfg.JWTSecret = firstNonEmpty(os.Getenv("JWT_SECRET"), fileCfgValue(fileCfg, func(c *Config) string { return c.JWTSecret }), "default_secret")
	cfg.UploadDir = firstNonEmpty(os.Getenv("UPLOAD_DIR"), fileCfgValue(fileCfg, func(c *Config) string { return c.UploadDir }), "uploads")

	return cfg
}

func loadFromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// helper to safely read from optional cfg
func fileCfgValue(cfg *Config, getter func(*Config) string) string {
	if cfg == nil {
		return ""
	}
	return getter(cfg)
}
