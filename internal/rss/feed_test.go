package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchLatestItems(t *testing.T) {
	// Create a test RSS feed
	testFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
	<channel>
		<title>Test Feed</title>
		<link>https://example.com</link>
		<description>A test feed</description>
		<item>
			<title>Test Item 1</title>
			<link>https://example.com/item1</link>
			<description>Description for item 1</description>
			<guid>item-1</guid>
			<pubDate>Mon, 02 Jan 2023 15:04:05 GMT</pubDate>
		</item>
		<item>
			<title>Test Item 2</title>
			<link>https://example.com/item2</link>
			<description>Description for item 2</description>
			<guid>item-2</guid>
			<pubDate>Mon, 02 Jan 2023 16:04:05 GMT</pubDate>
		</item>
		<item>
			<title>Test Item 3</title>
			<link>https://example.com/item3</link>
			<description>Description for item 3</description>
			<guid>item-3</guid>
			<pubDate>Mon, 02 Jan 2023 17:04:05 GMT</pubDate>
		</item>
	</channel>
</rss>`

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testFeed))
	}))
	defer server.Close()

	// Create checker
	checker := NewChecker()

	// Test fetching all items
	t.Run("FetchAllItems", func(t *testing.T) {
		items, err := checker.FetchLatestItems(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("Failed to fetch items: %v", err)
		}

		if len(items) != 3 {
			t.Errorf("Expected 3 items, got %d", len(items))
		}

		// Check first item
		if items[0].Title != "Test Item 1" {
			t.Errorf("Expected title 'Test Item 1', got '%s'", items[0].Title)
		}
		if items[0].Link != "https://example.com/item1" {
			t.Errorf("Expected link 'https://example.com/item1', got '%s'", items[0].Link)
		}
		if items[0].GUID != "item-1" {
			t.Errorf("Expected GUID 'item-1', got '%s'", items[0].GUID)
		}
	})

	// Test with context timeout
	t.Run("ContextTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond) // Ensure context is expired

		_, err := checker.FetchLatestItems(ctx, server.URL)
		if err == nil {
			t.Error("Expected error with expired context, got nil")
		}
	})
}

func TestFetchLatestItems_InvalidURL(t *testing.T) {
	checker := NewChecker()

	_, err := checker.FetchLatestItems(context.Background(), "not-a-valid-url")
	if err == nil {
		t.Error("Expected error with invalid URL, got nil")
	}
}

func TestFetchLatestItems_EmptyFeed(t *testing.T) {
	emptyFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
	<channel>
		<title>Empty Feed</title>
		<link>https://example.com</link>
		<description>A feed with no items</description>
	</channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(emptyFeed))
	}))
	defer server.Close()

	checker := NewChecker()
	items, err := checker.FetchLatestItems(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Expected no error with empty feed, got: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestFeedItem_GUIDFallback(t *testing.T) {
	// Feed with no GUID, should use link instead
	feedNoGUID := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
	<channel>
		<title>Test Feed</title>
		<link>https://example.com</link>
		<description>Test</description>
		<item>
			<title>Item Without GUID</title>
			<link>https://example.com/item-no-guid</link>
			<description>Description</description>
			<pubDate>Mon, 02 Jan 2023 15:04:05 GMT</pubDate>
		</item>
	</channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(feedNoGUID))
	}))
	defer server.Close()

	checker := NewChecker()
	items, err := checker.FetchLatestItems(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Failed to fetch items: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	// GUID should fall back to link
	if items[0].GUID != "https://example.com/item-no-guid" {
		t.Errorf("Expected GUID to fallback to link, got '%s'", items[0].GUID)
	}
}
