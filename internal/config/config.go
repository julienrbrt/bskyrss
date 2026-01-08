package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Accounts []Account     `yaml:"accounts"`
	Interval time.Duration `yaml:"interval,omitempty"`
	Storage  string        `yaml:"storage,omitempty"`
}

// Account represents a Bluesky account with its associated feeds
type Account struct {
	Handle   string   `yaml:"handle"`
	Password string   `yaml:"password,omitempty"`
	PDS      string   `yaml:"pds,omitempty"`
	Feeds    []string `yaml:"feeds"`
	Storage  string   `yaml:"storage,omitempty"` // Optional per-account storage file
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Process environment variables in passwords
	for i := range cfg.Accounts {
		cfg.Accounts[i].Password = expandEnvVar(cfg.Accounts[i].Password)
	}

	// Set defaults
	if cfg.Interval == 0 {
		cfg.Interval = 15 * time.Minute
	}
	if cfg.Storage == "" {
		cfg.Storage = "posted_items.txt"
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf("at least one account is required")
	}

	for i, account := range c.Accounts {
		if account.Handle == "" {
			return fmt.Errorf("account %d: handle is required", i)
		}
		if len(account.Feeds) == 0 {
			return fmt.Errorf("account %d (%s): at least one feed is required", i, account.Handle)
		}
		if account.Password == "" {
			return fmt.Errorf("account %d (%s): password is required", i, account.Handle)
		}
	}

	return nil
}

// expandEnvVar expands environment variable references in the format ${VAR_NAME} or $VAR_NAME
func expandEnvVar(value string) string {
	if value == "" {
		return value
	}

	// Handle ${VAR_NAME} format
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		varName := value[2 : len(value)-1]
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		return value
	}

	// Handle $VAR_NAME format
	if strings.HasPrefix(value, "$") {
		varName := value[1:]
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		return value
	}

	return value
}
