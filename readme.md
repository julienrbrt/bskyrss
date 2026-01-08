# Bluesky RSS Post

Tool to automatically post on Bluesky when new items are added to RSS/Atom feeds.

Supports:

- Multiple feeds posting to a single account
- Different feeds posting to different accounts
- Flexible configuration via YAML config file

## Installation

### Go Binary

```bash
go install pkg.rbrt.fr/bskyrss@latest
```

### Docker

```bash
docker run -d \
  --name bksyrss \
  -v $(pwd)/config.yaml:/config.yaml \
  -v $(pwd)/data:/data \
  -e BSKY_PASSWORD="your-app-password" \
  bksyrss \
  -config /config.yaml
```

## Usage

### Basic Setup

bskyrss requires a YAML configuration file to run. Create a `config.yaml` file:

```yaml
accounts:
  - handle: "your-handle.bsky.social"
    password: "${BSKY_PASSWORD}"
    feeds:
      - "https://example.com/feed.xml"
```

Set your password as an environment variable:

```bash
export BSKY_PASSWORD="your-app-password"
```

Run bskyrss:

```bash
./bskyrss -config config.yaml
```

### Multiple Feeds to One Account

```yaml
accounts:
  - handle: "your-handle.bsky.social"
    password: "${BSKY_PASSWORD}"
    feeds:
      - "https://example.com/feed.xml"
      - "https://blog.example.com/rss"
      - "https://news.example.com/atom.xml"
```

### Multiple Accounts with Different Feeds

Map different feeds to different Bluesky accounts:

```yaml
accounts:
  - handle: "tech-news.bsky.social"
    password: "${BSKY_PASSWORD_TECH}"
    feeds:
      - "https://hackernews.com/rss"
      - "https://techcrunch.com/feed"

  - handle: "personal.bsky.social"
    password: "${BSKY_PASSWORD_PERSONAL}"
    feeds:
      - "https://personal-blog.com/feed.xml"

interval: "15m"
```

Set environment variables:

```bash
export BSKY_PASSWORD_TECH="tech-app-password"
export BSKY_PASSWORD_PERSONAL="personal-app-password"
./bskyrss -config config.yaml
```

See [`config.example.yaml`](config.example.yaml) for a complete example with all options.

## Bluesky Authentication

### App Passwords (Recommended)

It's recommended to use an App Password instead of your main account password:

1. Go to Bluesky Settings → App Passwords
2. Create a new App Password
3. Use this password in your config file (via environment variable)

### Environment Variables

Passwords should be provided via environment variables for security:

```yaml
accounts:
  - handle: "user.bsky.social"
    password: "${BSKY_PASSWORD}" # References $BSKY_PASSWORD env var
```

Supported formats:

- `${VAR_NAME}` - Standard format
- `$VAR_NAME` - Short format

### Self-hosted PDS

If you're using a self-hosted Personal Data Server:

```yaml
accounts:
  - handle: "your-handle.your-pds.com"
    password: "${BSKY_PASSWORD}"
    pds: "https://your-pds.com"
    feeds:
      - "https://example.com/feed.xml"
```

## Post Format

Posts are formatted as follows:

```
{Title}

{Link}
```

- Posts are automatically truncated if they exceed Bluesky character limit
- Links are automatically detected and formatted as clickable links
- If the title is too long, it will be intelligently truncated at word boundaries

## Storage

The tool maintains a simple text file (default: `posted_items.txt`) containing the GUIDs of all posted items. This ensures that items are not posted multiple times, even if the tool is restarted.

The storage file contains one GUID per line and is safe to manually edit if needed.

### Multiple Accounts

When using multiple accounts, separate storage files are automatically created for each account (e.g., `posted_items_user1_bsky_social.txt`, `posted_items_user2_bsky_social.txt`). This ensures that each account tracks its own posted items independently.

You can also specify custom storage files per account in the configuration file:

```yaml
accounts:
  - handle: "user1.bsky.social"
    storage: "custom_storage_user1.txt"
    password: "${BSKY_PASSWORD_1}"
    feeds:
      - "https://feed1.com/rss"
```

### First Run Behavior

To prevent spam when starting the tool for the first time:

- **First Run (empty storage file)**: All existing feed items are marked as "seen" without posting. Only new items that appear _after_ the program starts will be posted.
- **Subsequent Runs**: New items since the last check are posted normally (backfill from last posted item).

This ensures you don't flood Bluesky with the entire feed history when you first set up the tool.

## License

[MIT](license).
