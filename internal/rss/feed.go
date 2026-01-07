package rss

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedItem represents a single RSS feed item
type FeedItem struct {
	GUID        string
	Title       string
	Description string
	Link        string
	Published   time.Time
}

// Checker handles RSS feed checking operations
type Checker struct {
	parser *gofeed.Parser
}

// NewChecker creates a new RSS feed checker
func NewChecker() *Checker {
	return &Checker{
		parser: gofeed.NewParser(),
	}
}

// FetchLatestItems fetches the latest items from an RSS feed
func (c *Checker) FetchLatestItems(ctx context.Context, feedURL string, limit int) ([]*FeedItem, error) {
	feed, err := c.parser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	if feed == nil || len(feed.Items) == 0 {
		return []*FeedItem{}, nil
	}

	// Limit the number of items to return
	maxItems := len(feed.Items)
	if limit > 0 && limit < maxItems {
		maxItems = limit
	}

	items := make([]*FeedItem, 0, maxItems)
	for i := 0; i < maxItems; i++ {
		item := feed.Items[i]

		published := time.Now()
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = *item.UpdatedParsed
		}

		// Use GUID if available, otherwise use link
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}

		items = append(items, &FeedItem{
			GUID:        guid,
			Title:       item.Title,
			Description: item.Description,
			Link:        item.Link,
			Published:   published,
		})
	}

	return items, nil
}
