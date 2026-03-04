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
	Defaults FeedOptions   `yaml:"defaults,omitempty"`
}

// Account represents a Bluesky account with its associated feeds
type Account struct {
	Handle   string       `yaml:"handle"`
	Password string       `yaml:"password,omitempty"`
	PDS      string       `yaml:"pds,omitempty"`
	Feeds    []FeedConfig `yaml:"feeds"`
	Storage  string       `yaml:"storage,omitempty"`
}

// FeedConfig holds a feed URL and optional per-feed HTTP/pacing options.
// In YAML it can be written as a plain string (just the URL) or as a mapping:
//
//	feeds:
//	  - "https://example.com/rss"
//	  - url: "https://old.reddit.com/r/linux/top/.rss?t=week"
//	    pre_fetch_delay: "5s"
//	    base_backoff: "30s"
type FeedConfig struct {
	URL     string      `yaml:"url"`
	Options FeedOptions `yaml:",inline"`
}

// UnmarshalYAML allows FeedConfig to be written as a plain string URL in YAML.
func (f *FeedConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		f.URL = value.Value
		return nil
	}

	// normal struct decoding
	type feedConfigAlias FeedConfig
	var alias feedConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*f = FeedConfig(alias)
	return nil
}

// FeedOptions controls HTTP behaviour, pacing and retry for a feed.
type FeedOptions struct {
	UserAgent string        `yaml:"user_agent,omitempty"`
	Timeout   time.Duration `yaml:"timeout,omitempty"`

	// Pre-fetch jitter.
	MinDelay time.Duration `yaml:"min_delay,omitempty"`
	MaxDelay time.Duration `yaml:"max_delay,omitempty"`

	// Retry / backoff on transient HTTP errors (5xx, 429 …)
	MaxRetries  *int          `yaml:"max_retries,omitempty"`
	BaseBackoff time.Duration `yaml:"base_backoff,omitempty"`
	MaxBackoff  time.Duration `yaml:"max_backoff,omitempty"`

	// When true (default), honour the Retry-After response header on HTTP 429.
	HonorRetryAfter *bool `yaml:"honor_retry_after,omitempty"`
}

// Resolved returns the effective FeedOptions for feed, with per-feed values
// taking priority over the global defaults block, which in turn takes priority
// over the hard-coded baseline.
func (c *Config) Resolved(feed FeedConfig) FeedOptions {
	f := feed.Options
	g := c.Defaults

	return FeedOptions{
		UserAgent:       coalesce(f.UserAgent, g.UserAgent, "bskyrss/1.0 (+https://pkg.rbrt.fr/bskyrss)"),
		Timeout:         coalesce(f.Timeout, g.Timeout, 30*time.Second),
		MinDelay:        coalesce(f.MinDelay, g.MinDelay, 0),
		MaxDelay:        coalesce(f.MaxDelay, g.MaxDelay, 0),
		MaxRetries:      coalesce(f.MaxRetries, g.MaxRetries, new(3)),
		BaseBackoff:     coalesce(f.BaseBackoff, g.BaseBackoff, 2*time.Second),
		MaxBackoff:      coalesce(f.MaxBackoff, g.MaxBackoff, 2*time.Minute),
		HonorRetryAfter: coalesce(f.HonorRetryAfter, g.HonorRetryAfter, new(true)),
	}
}

// coalesce returns the first value that is not the zero value of T.
func coalesce[T comparable](vals ...T) T {
	var zero T
	for _, v := range vals {
		if v != zero {
			return v
		}
	}
	return zero
}

// LoadFromFile loads configuration from a YAML file.
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

	// Expand environment variables in passwords
	for i := range cfg.Accounts {
		cfg.Accounts[i].Password = expandEnvVar(cfg.Accounts[i].Password)
	}

	// Global defaults
	if cfg.Interval == 0 {
		cfg.Interval = 15 * time.Minute
	}
	if cfg.Storage == "" {
		cfg.Storage = "posted_items.txt"
	}

	return &cfg, nil
}

// Validate checks the configuration for obvious errors.
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf("at least one account is required")
	}

	for i, account := range c.Accounts {
		if account.Handle == "" {
			return fmt.Errorf("account %d: handle is required", i)
		}
		if account.Password == "" {
			return fmt.Errorf("account %d (%s): password is required", i, account.Handle)
		}
		if len(account.Feeds) == 0 {
			return fmt.Errorf("account %d (%s): at least one feed is required", i, account.Handle)
		}
		for j, feed := range account.Feeds {
			if feed.URL == "" {
				return fmt.Errorf("account %d (%s), feed %d: url is required", i, account.Handle, j)
			}
		}
	}

	if err := validateFeedOptions("defaults", c.Defaults); err != nil {
		return err
	}

	return nil
}

func validateFeedOptions(prefix string, o FeedOptions) error {
	if o.Timeout < 0 {
		return fmt.Errorf("%s.timeout must be >= 0", prefix)
	}
	if o.MinDelay < 0 {
		return fmt.Errorf("%s.min_delay must be >= 0", prefix)
	}
	if o.MaxDelay < 0 {
		return fmt.Errorf("%s.max_delay must be >= 0", prefix)
	}
	if o.MinDelay > 0 && o.MaxDelay > 0 && o.MinDelay > o.MaxDelay {
		return fmt.Errorf("%s.min_delay must be <= max_delay", prefix)
	}
	if o.BaseBackoff < 0 {
		return fmt.Errorf("%s.base_backoff must be >= 0", prefix)
	}
	if o.MaxBackoff < 0 {
		return fmt.Errorf("%s.max_backoff must be >= 0", prefix)
	}
	if o.BaseBackoff > 0 && o.MaxBackoff > 0 && o.BaseBackoff > o.MaxBackoff {
		return fmt.Errorf("%s.base_backoff must be <= max_backoff", prefix)
	}
	if o.MaxRetries != nil && *o.MaxRetries < 0 {
		return fmt.Errorf("%s.max_retries must be >= 0", prefix)
	}
	return nil
}

// expandEnvVar expands ${VAR} or $VAR references.
func expandEnvVar(value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		varName := value[2 : len(value)-1]
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		return value
	}
	if strings.HasPrefix(value, "$") {
		varName := value[1:]
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		return value
	}
	return value
}
