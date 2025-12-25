# Scrq

[![License](https://img.shields.io/github/license/ahrdadan/scrq)](LICENSE)
[![Build](https://github.com/ahrdadan/scrq/actions/workflows/ci.yml/badge.svg)](https://github.com/ahrdadan/scrq/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ahrdadan/scrq)](https://github.com/ahrdadan/scrq/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ahrdadan/scrq)](go.mod)
[![Docker](https://img.shields.io/docker/pulls/ahrdadan/scrq)](https://hub.docker.com/r/ahrdadan/scrq)

**Scrq** (Scrape + Queue) — Asynchronous web scraping API built with Go, Fiber, Rod, and Lightpanda.

## ✨ Features

- 🚀 **Async Job Queue** - NATS JetStream for reliable job processing
- 🌐 **Dual Browser Support** - Lightpanda (lightweight) + Chrome (full-featured)
- 📡 **Real-time Updates** - WebSocket & SSE for live progress
- 🔔 **Webhook Notifications** - Get notified when jobs complete
- 📦 **Portable** - Auto-downloads NATS and Lightpanda binaries
- 🐳 **Docker Ready** - Multi-platform container support

## 🛠️ Tech Stack

| Component           | Technology                           |
| ------------------- | ------------------------------------ |
| Language            | Go 1.24                              |
| HTTP Server         | [Fiber](https://gofiber.io/) v2      |
| Browser Automation  | [Rod](https://go-rod.github.io/)     |
| Lightweight Browser | [Lightpanda](https://lightpanda.io/) |
| Message Queue       | [NATS](https://nats.io/) + JetStream |
| WebSocket           | Fiber WebSocket                      |

## 📦 Installation

### Binary Download

Download from [Releases](https://github.com/ahrdadan/scrq/releases):

```bash
# Linux
wget https://github.com/ahrdadan/scrq/releases/latest/download/scrq-linux-amd64
chmod +x scrq-linux-amd64
./scrq-linux-amd64
```

### Docker

```bash
docker pull ahrdadan/scrq:latest
docker run -p 8000:8000 ahrdadan/scrq:latest
```

### Build from Source

```bash
git clone https://github.com/ahrdadan/scrq.git
cd scrq
go build -o server ./cmd/server
./server
```

## 🚀 Quick Start

### Start the Server

```bash
./server --help

# Output:
# Scrq Server v1 (Scrape + Queue)
#
# Usage:
#   ./server [flags]
#
# Server:
#   --host            0.0.0.0
#   --port            8000
#
# Browser (Lightpanda CDP):
#   --browser-host    127.0.0.1
#   --browser-port    9222
#
# Chrome:
#   --with-chrome     false
#   --chrome-revision 0
#
# Queue (NATS JetStream):
#   --with-nats        true
#   --nats-url         nats://127.0.0.1:4222
#   --nats-store       ./data/nats
#   --nats-autodl      true
#   --nats-bin         ./bin/nats-server
#
# Other:
#   --version         show version
```

### Check Version

```bash
./server --version
# Scrq Server v1
```

### Basic Usage

```bash
# Start with defaults (Lightpanda + NATS)
./server

# With Chrome support
./server --with-chrome

# Custom port
./server --port 3000
```

## 📡 API Usage

### Create an Async Job

```bash
curl -X POST http://localhost:8000/scrq/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "scrape",
    "url": "https://example.com",
    "engine": "lightpanda",
    "script": "() => ({ title: document.title })"
  }'
```

Response (202 Accepted):

```json
{
  "success": true,
  "data": {
    "job_id": "job_abc123",
    "status": "queued",
    "status_url": "/scrq/jobs/job_abc123",
    "result_url": "/scrq/jobs/job_abc123/result",
    "events": {
      "sse_url": "/scrq/jobs/job_abc123/events",
      "ws_url": "/scrq/ws?job_id=job_abc123"
    }
  }
}
```

### Check Job Status

```bash
curl http://localhost:8000/scrq/jobs/job_abc123
```

### Get Job Result

```bash
curl http://localhost:8000/scrq/jobs/job_abc123/result
```

### Stream Events (SSE)

```bash
curl http://localhost:8000/scrq/jobs/job_abc123/events
```

### Synchronous Endpoints

For quick operations without the queue:

```bash
# Fetch page
curl -X POST http://localhost:8000/scrq/page/fetch \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'

# Take screenshot
curl -X POST http://localhost:8000/scrq/page/screenshot \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'

# Execute script
curl -X POST http://localhost:8000/scrq/page/evaluate \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "script": "() => document.title"
  }'
```

## 🐳 Docker

### Docker Compose

```yaml
version: "3.8"
services:
  scrq:
    image: ghcr.io/ahrdadan/scrq:latest
    ports:
      - "8000:8000"
    volumes:
      - scrq-data:/app/data
    command: ["--with-chrome"]
    shm_size: "2gb"

volumes:
  scrq-data:
```

### Build from Source

```bash
git clone https://github.com/ahrdadan/scrq.git
cd scrq
docker build -t scrq:latest .
docker run -p 8000:8000 scrq:latest
```

### Pull from GHCR

```bash
docker pull ghcr.io/ahrdadan/scrq:latest
docker run -p 8000:8000 ghcr.io/ahrdadan/scrq:latest
```

## 📚 Documentation

- [API Reference](docs/API.md)
- [Configuration](docs/CONFIGURATION.md)
- [Docker Deployment](docs/DOCKER.md)
- [Architecture](docs/ARCHITECTURE.md)

## 🔧 API Endpoints

| Method | Endpoint                 | Description         |
| ------ | ------------------------ | ------------------- |
| GET    | `/health`                | Health check        |
| GET    | `/scrq/browser/status`   | Browser status      |
| POST   | `/scrq/jobs`             | Create async job    |
| GET    | `/scrq/jobs/{id}`        | Get job status      |
| GET    | `/scrq/jobs/{id}/result` | Get job result      |
| POST   | `/scrq/jobs/{id}/cancel` | Cancel job          |
| GET    | `/scrq/jobs/{id}/events` | SSE stream          |
| GET    | `/scrq/ws?job_id={id}`   | WebSocket           |
| POST   | `/scrq/page/fetch`       | Fetch page (sync)   |
| POST   | `/scrq/page/screenshot`  | Screenshot (sync)   |
| POST   | `/scrq/page/evaluate`    | Run script (sync)   |
| POST   | `/scrq/scrape`           | Scrape page (sync)  |
| POST   | `/scrq/scrape/batch`     | Batch scrape (sync) |

Chrome endpoints available at `/scrq/chrome/*` when `--with-chrome` is enabled.

## ⚠️ Important Notes

### Lightpanda Browser

- **Linux only** - Lightpanda only supports Linux
- **Auto-download** - Binary is automatically downloaded if not present
- **Warning displayed** - If unavailable, a warning is shown and Lightpanda APIs are disabled

### Proxy Support

- Proxy is **only supported with Chrome engine**
- Use `--with-chrome` flag to enable Chrome
- Use `/scrq/chrome/*` endpoints for proxy

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Fiber](https://gofiber.io/) - Express inspired web framework
- [Rod](https://go-rod.github.io/) - High-level Go browser automation
- [Lightpanda](https://lightpanda.io/) - Lightweight headless browser
- [NATS](https://nats.io/) - Cloud native messaging system

With tags
go mod tidy
git add .
git commit -m "fix: add go mod tidy to CI workflows"
git push origin main
git tag -f v1.0.0
git push origin v1.0.0 --force
