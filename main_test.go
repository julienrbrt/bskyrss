package main

import (
	"testing"
)

func TestSanitizeHandle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple handle",
			input:    "user.bsky.social",
			expected: "user_bsky_social",
		},
		{
			name:     "handle with multiple dots",
			input:    "my.user.name.bsky.social",
			expected: "my_user_name_bsky_social",
		},
		{
			name:     "handle with @ prefix",
			input:    "@user.bsky.social",
			expected: "user_bsky_social",
		},
		{
			name:     "handle with multiple @ symbols",
			input:    "@@user.bsky.social",
			expected: "user_bsky_social",
		},
		{
			name:     "handle without dots",
			input:    "user",
			expected: "user",
		},
		{
			name:     "handle with dots and @",
			input:    "@test.user.example.com",
			expected: "test_user_example_com",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "@.@.",
			expected: "__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHandle(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeHandle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			text:     "short text",
			maxLen:   20,
			expected: "short text",
		},
		{
			name:     "truncate at word boundary",
			text:     "this is a long text that needs truncation",
			maxLen:   20,
			expected: "this is a long text",
		},
		{
			name:     "truncate without word boundary",
			text:     "verylongtextwithoutspaces",
			maxLen:   10,
			expected: "verylongte",
		},
		{
			name:     "exact length",
			text:     "exactly ten",
			maxLen:   11,
			expected: "exactly ten",
		},
		{
			name:     "empty string",
			text:     "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "truncate with space at end",
			text:     "text with space ",
			maxLen:   10,
			expected: "text with",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.text, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.maxLen, result, tt.expected)
			}
		})
	}
}
