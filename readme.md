# Bluesky RSS Post

Tool to automatically post on Bluesky when new items are added to RSS/Atom feeds.

## Installation

### Go Binary

```bash
go install pkg.rbrt.fr/bskyrss@latest
```

### Docker

```bash
docker run -d \
  --name bksyrss \
  -v $(pwd)/data:/data \
  -e BSKY_PASSWORD="your-app-password" \
  bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social" \
  -storage /data/posted_items.txt
```

## Usage

### Basic Usage

```bash
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social" \
  -password "your-app-password"
```

### Multiple Feeds

Monitor multiple feeds by separating URLs with commas:

```bash
./bksyrss \
  -feed "https://blog1.com/feed.xml,https://blog2.com/rss,https://news.com/atom.xml" \
  -handle "your-handle.bsky.social" \
  -password "your-app-password"
```

or

```bash
export BSKY_PASSWORD="your-app-password"
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social"
```

### Command Line Options

| Flag        | Description                                       | Required | Default             |
| ----------- | ------------------------------------------------- | -------- | ------------------- |
| `-feed`     | RSS/Atom feed URL(s) to monitor (comma-delimited) | Yes      | -                   |
| `-handle`   | Bluesky handle (e.g., user.bsky.social)           | Yes      | -                   |
| `-password` | Bluesky password or app password                  | Yes\*    | -                   |
| `-pds`      | Bluesky PDS server URL                            | No       | https://bsky.social |
| `-interval` | Poll interval for checking RSS feed               | No       | 15m                 |
| `-storage`  | File to store posted item GUIDs                   | No       | posted_items.txt    |
| `-dry-run`  | Don't post, just show what would be posted        | No       | false               |

\*Can be provided via `BSKY_PASSWORD` environment variable instead

### Examples

#### Monitor multiple feeds

```bash
./bksyrss \
  -feed "https://blog.com/rss,https://news.com/atom.xml,https://podcast.com/feed" \
  -handle "your-handle.bsky.social"
```

#### Check feeds every 5 minutes

```bash
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social" \
  -interval 5m
```

#### Test without posting (dry-run mode)

```bash
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social" \
  -dry-run
```

#### Use custom storage file

```bash
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.bsky.social" \
  -storage /var/lib/bksyrss/posted.txt
```

## Bluesky Authentication

### App Passwords (Recommended)

It's recommended to use an App Password instead of your main account password:

1. Go to Bluesky Settings → App Passwords
2. Create a new App Password
3. Use this password with the `-password` flag or `BSKY_PASSWORD` environment variable

### Self-hosted PDS

If you're using a self-hosted Personal Data Server:

```bash
./bksyrss \
  -feed "https://example.com/feed.xml" \
  -handle "your-handle.your-pds.com" \
  -pds "https://your-pds.com"
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

### First Run Behavior

To prevent spam when starting the tool for the first time:

- **First Run (empty storage file)**: All existing feed items are marked as "seen" without posting. Only new items that appear _after_ the program starts will be posted.
- **Subsequent Runs**: New items since the last check are posted normally (backfill from last posted item).

This ensures you don't flood Bluesky with the entire feed history when you first set up the tool.

## License

[MIT](license).
