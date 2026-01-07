package bluesky

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
)

// Client handles Bluesky API operations
type Client struct {
	xrpcClient *xrpc.Client
	handle     string
	did        string
	config     Config
	mu         sync.Mutex // protects token refresh
}

// Config holds configuration for Bluesky client
type Config struct {
	Handle   string
	Password string
	PDS      string // Personal Data Server URL (default: https://bsky.social)
}

// NewClient creates a new Bluesky client and authenticates
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.PDS == "" {
		cfg.PDS = "https://bsky.social"
	}

	xrpcClient := &xrpc.Client{
		Host: cfg.PDS,
	}

	// Authenticate
	auth, err := atproto.ServerCreateSession(ctx, xrpcClient, &atproto.ServerCreateSession_Input{
		Identifier: cfg.Handle,
		Password:   cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	xrpcClient.Auth = &xrpc.AuthInfo{
		AccessJwt:  auth.AccessJwt,
		RefreshJwt: auth.RefreshJwt,
		Handle:     auth.Handle,
		Did:        auth.Did,
	}

	return &Client{
		xrpcClient: xrpcClient,
		handle:     auth.Handle,
		did:        auth.Did,
		config:     cfg,
	}, nil
}

// Post creates a new post on Bluesky
func (c *Client) Post(ctx context.Context, text string) error {
	// Create the post record
	post := &bsky.FeedPost{
		Text:      text,
		CreatedAt: time.Now().Format(time.RFC3339),
		Langs:     []string{"en"},
	}

	// Detect and add facets for links
	facets := c.detectFacets(text)
	if len(facets) > 0 {
		post.Facets = facets
	}

	// Create the record
	input := &atproto.RepoCreateRecord_Input{
		Repo:       c.did,
		Collection: "app.bsky.feed.post",
		Record: &lexutil.LexiconTypeDecoder{
			Val: post,
		},
	}

	_, err := atproto.RepoCreateRecord(ctx, c.xrpcClient, input)
	if err != nil {
		// Check if token expired and retry once after refresh
		if c.isExpiredTokenError(err) {
			if refreshErr := c.refreshSession(ctx); refreshErr != nil {
				return fmt.Errorf("failed to create post: %w (refresh failed: %v)", err, refreshErr)
			}
			// Retry the post after refreshing
			_, err = atproto.RepoCreateRecord(ctx, c.xrpcClient, input)
			if err != nil {
				return fmt.Errorf("failed to create post after refresh: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// isExpiredTokenError checks if the error is due to an expired token
func (c *Client) isExpiredTokenError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "ExpiredToken") || strings.Contains(errStr, "Token has expired")
}

// refreshSession refreshes the authentication session
func (c *Client) refreshSession(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if someone else already refreshed while we were waiting
	if c.xrpcClient.Auth != nil && c.xrpcClient.Auth.RefreshJwt != "" {
		// Try to use the refresh token
		refresh, err := atproto.ServerRefreshSession(ctx, c.xrpcClient)
		if err == nil {
			c.xrpcClient.Auth.AccessJwt = refresh.AccessJwt
			c.xrpcClient.Auth.RefreshJwt = refresh.RefreshJwt
			return nil
		}
		// If refresh failed, fall through to re-authentication
	}

	// If refresh token doesn't work, re-authenticate with password
	auth, err := atproto.ServerCreateSession(ctx, c.xrpcClient, &atproto.ServerCreateSession_Input{
		Identifier: c.config.Handle,
		Password:   c.config.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to re-authenticate: %w", err)
	}

	c.xrpcClient.Auth = &xrpc.AuthInfo{
		AccessJwt:  auth.AccessJwt,
		RefreshJwt: auth.RefreshJwt,
		Handle:     auth.Handle,
		Did:        auth.Did,
	}

	return nil
}

// detectFacets detects links in text and creates facets for them
func (c *Client) detectFacets(text string) []*bsky.RichtextFacet {
	var facets []*bsky.RichtextFacet

	// Simple URL detection - looks for http:// or https://
	words := strings.Fields(text)
	currentPos := 0

	for _, word := range words {
		// Find the position of this word in the original text
		idx := strings.Index(text[currentPos:], word)
		if idx == -1 {
			continue
		}
		currentPos += idx

		// Check if it's a URL
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			// Clean up any trailing punctuation
			cleanURL := strings.TrimRight(word, ".,;:!?)")

			facet := &bsky.RichtextFacet{
				Index: &bsky.RichtextFacet_ByteSlice{
					ByteStart: int64(currentPos),
					ByteEnd:   int64(currentPos + len(cleanURL)),
				},
				Features: []*bsky.RichtextFacet_Features_Elem{
					{
						RichtextFacet_Link: &bsky.RichtextFacet_Link{
							Uri: cleanURL,
						},
					},
				},
			}
			facets = append(facets, facet)
		}

		currentPos += len(word)
	}

	return facets
}

// GetHandle returns the authenticated user's handle
func (c *Client) GetHandle() string {
	return c.handle
}

// GetDID returns the authenticated user's DID
func (c *Client) GetDID() string {
	return c.did
}
