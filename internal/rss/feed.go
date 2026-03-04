package rss

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"pkg.rbrt.fr/bskyrss/internal/config"
)

// FeedItem represents a single RSS feed item.
type FeedItem struct {
	GUID        string
	Title       string
	Description string
	Link        string
	Published   time.Time
}

// retryableStatuses are HTTP status codes that warrant a retry with backoff.
var retryableStatuses = map[int]struct{}{
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
	http.StatusRequestTimeout:      {},
}

const maxJitter = 750 * time.Millisecond

// capturingTransport stores the last response so we can read Retry-After,
// which gofeed.HTTPError does not expose.
type capturingTransport struct {
	inner     http.RoundTripper
	userAgent string
	lastResp  *http.Response
}

func (t *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	resp, err := t.inner.RoundTrip(req)
	if resp != nil {
		t.lastResp = resp
	}
	return resp, err
}

// Checker fetches RSS feeds. It is stateless; all behaviour is driven by the
// FeedOptions passed to FetchLatestItems.
type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

// FetchLatestItems fetches items from feedURL using opts.
// opts is expected to be fully resolved (via config.Config.Resolved).
func (c *Checker) FetchLatestItems(ctx context.Context, feedURL string, opts config.FeedOptions) ([]*FeedItem, error) {
	if opts.MaxDelay > 0 {
		lo := opts.MinDelay
		if lo > opts.MaxDelay {
			lo = 0
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled before fetch: %w", ctx.Err())
		case <-time.After(lo + randomDuration(opts.MaxDelay-lo)):
		}
	}

	transport := &capturingTransport{inner: http.DefaultTransport, userAgent: opts.UserAgent}
	parser := gofeed.NewParser()
	parser.Client = &http.Client{Timeout: opts.Timeout, Transport: transport}
	parser.UserAgent = opts.UserAgent

	honorRetryAfter := opts.HonorRetryAfter != nil && *opts.HonorRetryAfter
	maxRetries := 0
	if opts.MaxRetries != nil {
		maxRetries = *opts.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		feed, err := parser.ParseURLWithContext(feedURL, ctx)
		if err == nil {
			return itemsFromFeed(feed), nil
		}
		lastErr = err
		if attempt == maxRetries {
			break
		}
		wait, shouldRetry := retryWait(err, attempt, opts.BaseBackoff, opts.MaxBackoff, honorRetryAfter, transport.lastResp)
		if !shouldRetry {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(wait):
		}
	}

	return nil, fmt.Errorf("failed to parse RSS feed: %w", lastErr)
}

func itemsFromFeed(feed *gofeed.Feed) []*FeedItem {
	if feed == nil || len(feed.Items) == 0 {
		return []*FeedItem{}
	}

	items := make([]*FeedItem, 0, len(feed.Items))
	for _, item := range feed.Items {
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

	return items
}

// retryWait returns the wait duration and whether the request should be retried.
func retryWait(err error, attempt int, base, max time.Duration, honorRetryAfter bool, lastResp *http.Response) (time.Duration, bool) {
	var httpErr gofeed.HTTPError
	if !errors.As(err, &httpErr) {
		return 0, false
	}
	if _, ok := retryableStatuses[httpErr.StatusCode]; !ok {
		return 0, false
	}
	if honorRetryAfter && httpErr.StatusCode == http.StatusTooManyRequests {
		if d, ok := parseRetryAfter(lastResp, max); ok {
			return d + randomDuration(maxJitter), true
		}
	}
	return min(base*time.Duration(1<<attempt), max) + randomDuration(maxJitter), true
}

// parseRetryAfter parses the Retry-After header, capped at cap.
func parseRetryAfter(resp *http.Response, cap time.Duration) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs >= 0 {
		return min(time.Duration(secs)*time.Second, cap), true
	}
	if t, err := http.ParseTime(raw); err == nil {
		return min(max(time.Until(t), 0), cap), true
	}
	return 0, false
}

func randomDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max) + 1))
}
