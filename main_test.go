package main

import (
	"reflect"
	"testing"
)

func TestParseFeedURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single feed",
			input:    "https://example.com/feed.xml",
			expected: []string{"https://example.com/feed.xml"},
		},
		{
			name:     "multiple feeds",
			input:    "https://example.com/feed1.xml,https://example.com/feed2.xml",
			expected: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
		},
		{
			name:     "multiple feeds with spaces",
			input:    "https://example.com/feed1.xml, https://example.com/feed2.xml, https://example.com/feed3.xml",
			expected: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml", "https://example.com/feed3.xml"},
		},
		{
			name:     "feeds with extra whitespace",
			input:    "  https://example.com/feed1.xml  ,  https://example.com/feed2.xml  ",
			expected: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only commas and spaces",
			input:    " , , , ",
			expected: []string{},
		},
		{
			name:     "trailing comma",
			input:    "https://example.com/feed1.xml,https://example.com/feed2.xml,",
			expected: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
		},
		{
			name:     "leading comma",
			input:    ",https://example.com/feed1.xml,https://example.com/feed2.xml",
			expected: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
		},
		{
			name:     "mixed RSS and Atom feeds",
			input:    "https://blog.com/rss,https://news.com/atom.xml,https://site.com/feed",
			expected: []string{"https://blog.com/rss", "https://news.com/atom.xml", "https://site.com/feed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFeedURLs(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseFeedURLs(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFeedURLsCount(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
	}{
		{"one feed", "https://example.com/feed.xml", 1},
		{"two feeds", "https://a.com/feed,https://b.com/feed", 2},
		{"five feeds", "https://1.com/f,https://2.com/f,https://3.com/f,https://4.com/f,https://5.com/f", 5},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFeedURLs(tt.input)
			if len(result) != tt.expectedCount {
				t.Errorf("parseFeedURLs(%q) returned %d feeds, want %d", tt.input, len(result), tt.expectedCount)
			}
		})
	}
}
