package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pkg.rbrt.fr/bskyrss/internal/bluesky"
	"pkg.rbrt.fr/bskyrss/internal/config"
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
	configFile := flag.String("config", "config.yaml", "Path to configuration file (YAML)")
	dryRun := flag.Bool("dry-run", false, "Don't actually post to Bluesky, just show what would be posted")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Error: -config flag is required")
	}

	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}
	log.Printf("Loaded configuration with %d account(s)", len(cfg.Accounts))

	if *dryRun {
		log.Println("Running in DRY-RUN mode - no posts will be made")
	}

	// Count total feeds across all accounts
	totalFeeds := 0
	for _, account := range cfg.Accounts {
		totalFeeds += len(account.Feeds)
	}
	log.Printf("Monitoring %d feed(s) across %d account(s)", totalFeeds, len(cfg.Accounts))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Initialize managers for each account
	managers := make([]*AccountManager, 0, len(cfg.Accounts))
	for i, account := range cfg.Accounts {
		storageFilePath := cfg.Storage
		if account.Storage != "" {
			storageFilePath = account.Storage
		} else if len(cfg.Accounts) > 1 {
			// For multiple accounts, use separate storage files by default
			ext := ".txt"
			base := strings.TrimSuffix(cfg.Storage, ext)
			storageFilePath = fmt.Sprintf("%s_%s%s", base, sanitizeHandle(account.Handle), ext)
		}

		manager, err := NewAccountManager(ctx, cfg, account, storageFilePath, *dryRun)
		if err != nil {
			log.Fatalf("Failed to initialize account %d (%s): %v", i+1, account.Handle, err)
		}
		managers = append(managers, manager)
		log.Printf("Initialized account: @%s with %d feed(s) (storage: %s)", account.Handle, len(account.Feeds), storageFilePath)
	}

	log.Printf("Poll interval: %s", cfg.Interval)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	// Check immediately on startup
	for _, manager := range managers {
		if err := manager.CheckAndPost(ctx); err != nil {
			log.Printf("Error during initial check for @%s: %v", manager.account.Handle, err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, manager := range managers {
				if err := manager.CheckAndPost(ctx); err != nil {
					log.Printf("Error during check for @%s: %v", manager.account.Handle, err)
				}
			}
		}
	}
}

// AccountManager manages RSS checking and posting for a single Bluesky account.
type AccountManager struct {
	cfg        *config.Config
	account    config.Account
	bskyClient *bluesky.Client
	rssChecker *rss.Checker
	store      *storage.Storage
	dryRun     bool
}

// NewAccountManager creates a new AccountManager.
func NewAccountManager(ctx context.Context, cfg *config.Config, account config.Account, storageFile string, dryRun bool) (*AccountManager, error) {
	store, err := storage.New(storageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	rssChecker := rss.NewChecker()

	var bskyClient *bluesky.Client
	if !dryRun {
		authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		pds := account.PDS
		if pds == "" {
			pds = "https://bsky.social"
		}

		bskyClient, err = bluesky.NewClient(authCtx, bluesky.Config{
			Handle:   account.Handle,
			Password: account.Password,
			PDS:      pds,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Bluesky client: %w", err)
		}
	}

	return &AccountManager{
		cfg:        cfg,
		account:    account,
		bskyClient: bskyClient,
		rssChecker: rssChecker,
		store:      store,
		dryRun:     dryRun,
	}, nil
}

// CheckAndPost checks all feeds for this account and posts new items.
func (m *AccountManager) CheckAndPost(ctx context.Context) error {
	for i, feed := range m.account.Feeds {
		if err := m.checkAndPostFeed(ctx, feed); err != nil {
			log.Printf("[@%s] Error checking feed %s: %v", m.account.Handle, feed.URL, err)
			// Continue with other feeds even if one fails.
		}
		if i < len(m.account.Feeds)-1 {
			opts := m.cfg.Resolved(feed)
			if opts.MinDelay > 0 {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(opts.MinDelay):
				}
			}
		}
	}
	return nil
}

// checkAndPostFeed checks a single feed and posts new items.
func (m *AccountManager) checkAndPostFeed(ctx context.Context, feed config.FeedConfig) error {
	log.Printf("[@%s] Checking RSS feed: %s", m.account.Handle, feed.URL)

	opts := m.cfg.Resolved(feed)

	items, err := m.rssChecker.FetchLatestItems(ctx, feed.URL, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS items: %w", err)
	}

	log.Printf("[@%s] Found %d items in feed", m.account.Handle, len(items))

	// Check if this is the first time seeing this feed (no items in storage).
	hasSeenFeedBefore := false
	for _, item := range items {
		if m.store.IsPosted(item.GUID) {
			hasSeenFeedBefore = true
			break
		}
	}

	// Process items in reverse order (oldest first).
	newItemCount := 0
	postedCount := 0

	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]

		if m.store.IsPosted(item.GUID) {
			continue
		}

		newItemCount++

		// First time seeing this feed: mark items as seen without posting.
		if !hasSeenFeedBefore {
			if err := m.store.MarkPosted(item.GUID); err != nil {
				log.Printf("[@%s] Failed to mark item as seen: %v", m.account.Handle, err)
			}
			continue
		}

		log.Printf("[@%s] New item found: %s", m.account.Handle, item.Title)

		postText := formatPost(item)

		if m.dryRun {
			log.Printf("[@%s] [DRY-RUN] Would post:\n%s\n", m.account.Handle, postText)
			postedCount++
		} else {
			postCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := m.bskyClient.Post(postCtx, postText)
			cancel()

			if err != nil {
				log.Printf("[@%s] Failed to post item '%s': %v", m.account.Handle, item.Title, err)
				continue
			}

			log.Printf("[@%s] Successfully posted: %s", m.account.Handle, item.Title)
			postedCount++
		}

		if err := m.store.MarkPosted(item.GUID); err != nil {
			log.Printf("[@%s] Failed to mark item as posted: %v", m.account.Handle, err)
		}

		// Small self-imposed delay between posts to avoid Bluesky rate limits.
		if postedCount > 0 && !m.dryRun {
			time.Sleep(2 * time.Second)
		}
	}

	if !hasSeenFeedBefore {
		if newItemCount > 0 {
			log.Printf("[@%s] New feed detected: marked %d items as seen from %s (not posted)", m.account.Handle, newItemCount, feed.URL)
		}
	} else {
		if newItemCount == 0 {
			log.Printf("[@%s] No new items in feed %s", m.account.Handle, feed.URL)
		} else {
			log.Printf("[@%s] Processed %d new items from feed %s (%d posted)", m.account.Handle, newItemCount, feed.URL, postedCount)
		}
	}

	return nil
}

func formatPost(item *rss.FeedItem) string {
	urls := []string{}
	if item.Link != "" {
		urls = append(urls, item.Link)
	}

	// Add GUID if it's a URL and different from link (e.g. HN comment links).
	if item.GUID != "" && item.GUID != item.Link {
		if u, err := url.Parse(item.GUID); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			urls = append(urls, item.GUID)
		}
	}

	text := item.Title
	if len(urls) > 0 {
		text += "\n" + strings.Join(urls, "\n")
	}

	if len(text) > maxPostLength {
		linkText := ""
		if len(urls) > 0 {
			linkText = "\n" + strings.Join(urls, "\n")
		}

		availableForTitle := maxPostLength - len(linkText) - 3 // 3 for "..."
		if availableForTitle > 20 {
			text = truncateText(item.Title, availableForTitle) + "..." + linkText
		} else {
			if len(urls) > 0 {
				text = urls[0]
			} else {
				text = truncateText(item.Title, maxPostLength-3) + "..."
			}
		}
	}

	return text
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	truncated := text[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		return text[:lastSpace]
	}
	return truncated
}

// sanitizeHandle replaces characters unsuitable for filenames.
func sanitizeHandle(handle string) string {
	sanitized := strings.ReplaceAll(handle, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "@", "")
	return sanitized
}
