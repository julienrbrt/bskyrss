package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pkg.rbrt.fr/bskyrss/internal/bluesky"
	"pkg.rbrt.fr/bskyrss/internal/rss"
	"pkg.rbrt.fr/bskyrss/internal/storage"
)

const (
	defaultPollInterval = 15 * time.Minute
	defaultStorageFile  = "posted_items.txt"
	maxPostLength       = 299 // Bluesky has a 299 character limit
)

func main() {
	// Command line flags
	feedURLs := flag.String("feed", "", "RSS/Atom feed URL(s) to monitor (comma-delimited for multiple feeds, required)")
	handle := flag.String("handle", "", "Bluesky handle (required)")
	password := flag.String("password", "", "Bluesky password (can also use BSKY_PASSWORD env var)")
	pds := flag.String("pds", "https://bsky.social", "Bluesky PDS server URL")
	pollInterval := flag.Duration("interval", defaultPollInterval, "Poll interval for checking RSS feed")
	storageFile := flag.String("storage", defaultStorageFile, "File to store posted item GUIDs")
	dryRun := flag.Bool("dry-run", false, "Don't actually post to Bluesky, just show what would be posted")
	flag.Parse()

	// Validate required flags
	if *feedURLs == "" {
		log.Fatal("Error: -feed flag is required")
	}
	if *handle == "" {
		log.Fatal("Error: -handle flag is required")
	}

	// Parse comma-delimited feed URLs
	feeds := parseFeedURLs(*feedURLs)
	if len(feeds) == 0 {
		log.Fatal("Error: no valid feed URLs provided")
	}
	log.Printf("Monitoring %d feed(s)", len(feeds))

	// Get password from flag or environment variable
	bskyPassword := *password
	if bskyPassword == "" {
		bskyPassword = os.Getenv("BSKY_PASSWORD")
		if bskyPassword == "" {
			log.Fatal("Error: -password flag or BSKY_PASSWORD environment variable is required")
		}
	}

	store, err := storage.New(*storageFile)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Determine if this is the first run (no items in storage)
	isFirstRun := store.Count() == 0
	if isFirstRun {
		log.Println("First run detected - will mark existing items as seen without posting")
	} else {
		log.Printf("Storage initialized with %d previously posted items", store.Count())
	}

	rssChecker := rss.NewChecker()

	// Initialize Bluesky client (unless dry-run mode)
	var bskyClient *bluesky.Client
	if !*dryRun {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		bskyClient, err = bluesky.NewClient(ctx, bluesky.Config{
			Handle:   *handle,
			Password: bskyPassword,
			PDS:      *pds,
		})
		if err != nil {
			log.Fatalf("Failed to initialize Bluesky client: %v", err)
		}
		log.Printf("Authenticated as @%s", bskyClient.GetHandle())
	} else {
		log.Println("Running in DRY-RUN mode - no posts will be made")
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Main loop
	log.Printf("Poll interval: %s", *pollInterval)

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	// Check immediately on startup
	if err := checkAndPostFeeds(ctx, rssChecker, bskyClient, store, feeds, *dryRun, isFirstRun); err != nil {
		log.Printf("Error during initial check: %v", err)
	}

	// Continue checking on interval (not first run anymore after first check)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := checkAndPostFeeds(ctx, rssChecker, bskyClient, store, feeds, *dryRun, false); err != nil {
				log.Printf("Error during check: %v", err)
			}
		}
	}
}

// parseFeedURLs splits comma-delimited feed URLs and trims whitespace
func parseFeedURLs(feedString string) []string {
	if feedString == "" {
		return nil
	}

	parts := strings.Split(feedString, ",")
	feeds := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			feeds = append(feeds, trimmed)
		}
	}

	return feeds
}

func checkAndPostFeeds(ctx context.Context, rssChecker *rss.Checker, bskyClient *bluesky.Client, store *storage.Storage, feedURLs []string, dryRun bool, isFirstRun bool) error {
	for _, feedURL := range feedURLs {
		if err := checkAndPost(ctx, rssChecker, bskyClient, store, feedURL, dryRun, isFirstRun); err != nil {
			log.Printf("Error checking feed %s: %v", feedURL, err)
			// Continue with other feeds even if one fails
		}
	}

	return nil
}

func checkAndPost(
	ctx context.Context,
	rssChecker *rss.Checker,
	bskyClient *bluesky.Client,
	store *storage.Storage,
	feedURL string,
	dryRun bool,
	isFirstRun bool,
) error {
	log.Printf("Checking RSS feed: %s", feedURL)

	limit := 20
	items, err := rssChecker.FetchLatestItems(ctx, feedURL, limit)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS items: %w", err)
	}

	log.Printf("Found %d items in feed", len(items))

	// Process items in reverse order (oldest first)
	newItemCount := 0
	postedCount := 0

	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]

		// Skip if already posted
		if store.IsPosted(item.GUID) {
			continue
		}

		newItemCount++

		// On first run, just mark items as seen without posting
		if isFirstRun {
			if err := store.MarkPosted(item.GUID); err != nil {
				log.Printf("Failed to mark item as seen: %v", err)
			}
			continue
		}

		log.Printf("New item found: %s", item.Title)

		// Create post text
		postText := formatPost(item)

		if dryRun {
			log.Printf("[DRY-RUN] Would post:\n%s\n", postText)
			postedCount++
		} else {
			// Post to Bluesky
			postCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := bskyClient.Post(postCtx, postText)
			cancel()

			if err != nil {
				log.Printf("Failed to post item '%s': %v", item.Title, err)
				continue
			}

			log.Printf("Successfully posted: %s", item.Title)
			postedCount++
		}

		// Mark as posted
		if err := store.MarkPosted(item.GUID); err != nil {
			log.Printf("Failed to mark item as posted: %v", err)
		}

		// Rate limiting - wait a bit between posts to avoid overwhelming Bluesky
		if postedCount > 0 && !dryRun && !isFirstRun {
			time.Sleep(2 * time.Second)
		}
	}

	if isFirstRun {
		if newItemCount > 0 {
			log.Printf("Marked %d items as seen from feed %s", newItemCount, feedURL)
		}
	} else {
		if newItemCount == 0 {
			log.Printf("No new items in feed %s", feedURL)
		} else {
			log.Printf("Processed %d new items from feed %s (%d posted)", newItemCount, feedURL, postedCount)
		}
	}

	return nil
}

func formatPost(item *rss.FeedItem) string {
	// Start with title
	text := item.Title

	// Add link if available
	if item.Link != "" {
		text += "\n\n" + item.Link
	}

	// Truncate if too long
	if len(text) > maxPostLength {
		// Try to truncate title intelligently
		maxTitleLen := maxPostLength - len(item.Link) - 5 // 5 for "\n\n" and "..."
		if maxTitleLen > 0 {
			text = truncateText(item.Title, maxTitleLen) + "...\n\n" + item.Link
		} else {
			// If even with minimal title it's too long, just use the link
			text = item.Link
		}
	}

	return text
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Try to truncate at word boundary
	truncated := text[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		return text[:lastSpace]
	}

	return truncated
}
