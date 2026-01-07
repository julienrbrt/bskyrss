package bluesky

import (
	"context"
	"fmt"
	"strings"
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
		return fmt.Errorf("failed to create post: %w", err)
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
