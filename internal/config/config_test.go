package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    pds: "https://bsky.social"
    feeds:
      - "https://feed1.com/rss"
      - "https://feed2.com/atom"
  - handle: "user2.bsky.social"
    password: "password2"
    feeds:
      - "https://feed3.com/rss"

interval: "10m"
storage: "custom_storage.txt"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if len(cfg.Accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(cfg.Accounts))
	}

	if cfg.Accounts[0].Handle != "user1.bsky.social" {
		t.Errorf("Expected handle 'user1.bsky.social', got '%s'", cfg.Accounts[0].Handle)
	}
	if cfg.Accounts[0].Password != "password1" {
		t.Errorf("Expected password 'password1', got '%s'", cfg.Accounts[0].Password)
	}
	if len(cfg.Accounts[0].Feeds) != 2 {
		t.Errorf("Expected 2 feeds for account 1, got %d", len(cfg.Accounts[0].Feeds))
	}
	if cfg.Accounts[0].Feeds[0].URL != "https://feed1.com/rss" {
		t.Errorf("Expected feed URL 'https://feed1.com/rss', got '%s'", cfg.Accounts[0].Feeds[0].URL)
	}
	if cfg.Accounts[0].Feeds[1].URL != "https://feed2.com/atom" {
		t.Errorf("Expected feed URL 'https://feed2.com/atom', got '%s'", cfg.Accounts[0].Feeds[1].URL)
	}

	if cfg.Accounts[1].Handle != "user2.bsky.social" {
		t.Errorf("Expected handle 'user2.bsky.social', got '%s'", cfg.Accounts[1].Handle)
	}
	if len(cfg.Accounts[1].Feeds) != 1 {
		t.Errorf("Expected 1 feed for account 2, got %d", len(cfg.Accounts[1].Feeds))
	}

	if cfg.Interval != 10*time.Minute {
		t.Errorf("Expected interval 10m, got %v", cfg.Interval)
	}
	if cfg.Storage != "custom_storage.txt" {
		t.Errorf("Expected storage 'custom_storage.txt', got '%s'", cfg.Storage)
	}
}

func TestLoadFromFile_FeedConfigMapping(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds:
      - "https://plain-url.com/rss"
      - url: "https://mapping-url.com/rss"
        user_agent: "custom-agent/1.0"
        base_backoff: "10s"
        max_backoff: "5m"
        honor_retry_after: false
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	feeds := cfg.Accounts[0].Feeds
	if len(feeds) != 2 {
		t.Fatalf("Expected 2 feeds, got %d", len(feeds))
	}

	// Plain string feed
	if feeds[0].URL != "https://plain-url.com/rss" {
		t.Errorf("Expected URL 'https://plain-url.com/rss', got '%s'", feeds[0].URL)
	}
	if feeds[0].Options.UserAgent != "" {
		t.Errorf("Expected empty UserAgent for plain feed, got '%s'", feeds[0].Options.UserAgent)
	}

	// Mapping feed with overrides
	if feeds[1].URL != "https://mapping-url.com/rss" {
		t.Errorf("Expected URL 'https://mapping-url.com/rss', got '%s'", feeds[1].URL)
	}
	if feeds[1].Options.UserAgent != "custom-agent/1.0" {
		t.Errorf("Expected UserAgent 'custom-agent/1.0', got '%s'", feeds[1].Options.UserAgent)
	}
	if feeds[1].Options.BaseBackoff != 10*time.Second {
		t.Errorf("Expected BaseBackoff 10s, got %v", feeds[1].Options.BaseBackoff)
	}
	if feeds[1].Options.MaxBackoff != 5*time.Minute {
		t.Errorf("Expected MaxBackoff 5m, got %v", feeds[1].Options.MaxBackoff)
	}
	if feeds[1].Options.HonorRetryAfter == nil || *feeds[1].Options.HonorRetryAfter != false {
		t.Errorf("Expected HonorRetryAfter false, got %v", feeds[1].Options.HonorRetryAfter)
	}
}

func TestLoadFromFile_Resolved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds:
      - "https://feed1.com/rss"
      - url: "https://feed2.com/rss"
        user_agent: "per-feed-agent/1.0"

defaults:
  user_agent: "global-agent/1.0"
  base_backoff: "10s"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Plain feed: should inherit global defaults
	plain := cfg.Resolved(cfg.Accounts[0].Feeds[0])
	if plain.UserAgent != "global-agent/1.0" {
		t.Errorf("Expected global UserAgent for plain feed, got '%s'", plain.UserAgent)
	}
	// Timeout should fall back to hard-coded default
	if plain.Timeout != 30*time.Second {
		t.Errorf("Expected default Timeout 30s, got %v", plain.Timeout)
	}

	// Per-feed override wins over global default
	overridden := cfg.Resolved(cfg.Accounts[0].Feeds[1])
	if overridden.UserAgent != "per-feed-agent/1.0" {
		t.Errorf("Expected per-feed UserAgent, got '%s'", overridden.UserAgent)
	}
	// BaseBackoff should be inherited from global defaults
	if overridden.BaseBackoff != 10*time.Second {
		t.Errorf("Expected BaseBackoff 10s inherited from global, got %v", overridden.BaseBackoff)
	}
}

func TestLoadFromFileWithEnvVars(t *testing.T) {
	os.Setenv("TEST_PASSWORD", "env-password")
	defer os.Unsetenv("TEST_PASSWORD")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
accounts:
  - handle: "user1.bsky.social"
    password: "${TEST_PASSWORD}"
    feeds:
      - "https://feed1.com/rss"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Accounts[0].Password != "env-password" {
		t.Errorf("Expected password 'env-password' from env var, got '%s'", cfg.Accounts[0].Password)
	}
}

func TestLoadFromFileDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds:
      - "https://feed1.com/rss"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Interval != 15*time.Minute {
		t.Errorf("Expected default interval 15m, got %v", cfg.Interval)
	}
	if cfg.Storage != "posted_items.txt" {
		t.Errorf("Expected default storage 'posted_items.txt', got '%s'", cfg.Storage)
	}
}

func TestLoadFromFileInvalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "no accounts",
			content: `
interval: "15m"
`,
			wantErr: true,
		},
		{
			name: "account without handle",
			content: `
accounts:
  - password: "password1"
    feeds:
      - "https://feed1.com/rss"
`,
			wantErr: true,
		},
		{
			name: "account without password",
			content: `
accounts:
  - handle: "user1.bsky.social"
    feeds:
      - "https://feed1.com/rss"
`,
			wantErr: true,
		},
		{
			name: "account without feeds",
			content: `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds: []
`,
			wantErr: true,
		},
		{
			name: "feed with empty url in mapping",
			content: `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds:
      - url: ""
`,
			wantErr: true,
		},
		{
			name: "defaults base_backoff > max_backoff",
			content: `
accounts:
  - handle: "user1.bsky.social"
    password: "password1"
    feeds:
      - "https://feed1.com/rss"
defaults:
  base_backoff: "10m"
  max_backoff: "1m"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err := LoadFromFile(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envKey   string
		envValue string
		want     string
	}{
		{
			name:     "expand ${VAR}",
			input:    "${TEST_VAR}",
			envKey:   "TEST_VAR",
			envValue: "test-value",
			want:     "test-value",
		},
		{
			name:     "expand $VAR",
			input:    "$TEST_VAR",
			envKey:   "TEST_VAR",
			envValue: "test-value",
			want:     "test-value",
		},
		{
			name:     "no expansion plain text",
			input:    "plain-text",
			envKey:   "TEST_VAR",
			envValue: "test-value",
			want:     "plain-text",
		},
		{
			name:     "env var not set ${VAR}",
			input:    "${NONEXISTENT}",
			envKey:   "OTHER_VAR",
			envValue: "test-value",
			want:     "${NONEXISTENT}",
		},
		{
			name:     "env var not set $VAR",
			input:    "$NONEXISTENT",
			envKey:   "OTHER_VAR",
			envValue: "test-value",
			want:     "$NONEXISTENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			got := expandEnvVar(tt.input)
			if got != tt.want {
				t.Errorf("expandEnvVar(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
