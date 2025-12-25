# Scrq Configuration

## CLI Flags

Scrq supports the following command-line flags:

```bash
./server [flags]
```

### Server Configuration

| Flag         | Default                   | Description                                            |
| ------------ | ------------------------- | ------------------------------------------------------ |
| `--host`     | `0.0.0.0`                 | Host address to bind the server                        |
| `--port`     | `8000`                    | Port number for the server                             |
| `--base-url` | `http://localhost:8000`   | Base URL for full URLs in API responses (auto-detect)  |

### Browser (Lightpanda CDP)

| Flag             | Default     | Description                 |
| ---------------- | ----------- | --------------------------- |
| `--browser-host` | `127.0.0.1` | Lightpanda browser CDP host |
| `--browser-port` | `9222`      | Lightpanda browser CDP port |

### Chrome

| Flag                | Default | Description                                        |
| ------------------- | ------- | -------------------------------------------------- |
| `--with-chrome`     | `false` | Download Chrome and enable Chrome-backed endpoints |
| `--chrome-revision` | `0`     | Chromium revision to download (0 uses default)     |

### Queue (NATS JetStream)

| Flag            | Default                 | Description                         |
| --------------- | ----------------------- | ----------------------------------- |
| `--with-nats`   | `true`                  | Enable NATS JetStream for job queue |
| `--nats-url`    | `nats://127.0.0.1:4222` | NATS server URL                     |
| `--nats-store`  | `./data/nats`           | NATS JetStream storage directory    |
| `--nats-autodl` | `true`                  | Auto-download NATS server binary    |
| `--nats-bin`    | `./bin/nats-server`     | Path to NATS server binary          |

### Other

| Flag        | Default | Description              |
| ----------- | ------- | ------------------------ |
| `--version` | -       | Show version information |
| `--help`    | -       | Show help message        |

## Examples

### Basic Usage

```bash
# Start with defaults
./server

# Custom port
./server --port 3000

# With Chrome support
./server --with-chrome

# Without NATS (sync mode only)
./server --with-nats=false
```

### Production Setup

```bash
./server \
  --host 0.0.0.0 \
  --port 8000 \
  --with-chrome \
  --nats-store /var/lib/scrq/nats
```

### Docker

```bash
docker run -p 8000:8000 ahrdadan/scrq:latest
```

## Environment Variables

You can also configure Scrq using environment variables (coming soon):

| Variable         | Flag Equivalent |
| ---------------- | --------------- |
| `SCRQ_HOST`      | `--host`        |
| `SCRQ_PORT`      | `--port`        |
| `SCRQ_WITH_NATS` | `--with-nats`   |
| `SCRQ_NATS_URL`  | `--nats-url`    |

## Auto-Download Features

### NATS Server

When `--with-nats` is enabled and `--nats-autodl` is true:

1. Scrq checks if nats-server binary exists at `--nats-bin`
2. If not found, downloads the appropriate version for your OS/arch
3. Starts the NATS server with JetStream enabled

Supported platforms:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Lightpanda Browser

On Linux:

1. Scrq checks for Lightpanda binary in `./browser/` directory
2. If not found, automatically downloads from GitHub releases
3. Makes the binary executable

On other platforms:

- Lightpanda is not available
- A warning is logged
- Lightpanda-related APIs are disabled
- Chrome endpoints remain available if `--with-chrome` is set
