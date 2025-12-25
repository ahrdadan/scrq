# Docker Deployment

## Quick Start

### Using Docker

```bash
# Pull the image
docker pull ahrdadan/scrq:latest

# Run
docker run -p 8000:8000 ahrdadan/scrq:latest
```

### Using Docker Compose

```bash
docker-compose up -d
```

## Building from Source

### Clone Repository

```bash
git clone https://github.com/ahrdadan/scrq.git
cd scrq
```

### Build Docker Image

```bash
docker build -t scrq:latest .
```

### Run

```bash
docker run -p 8000:8000 scrq:latest
```

## Dockerfile

The Dockerfile uses a multi-stage build:

1. **Builder stage**: Compiles the Go binary
2. **Runtime stage**: Minimal Debian image with the binary

### Build Arguments

| Argument     | Description                     |
| ------------ | ------------------------------- |
| `VERSION`    | Version string (default: `dev`) |
| `BUILD_TIME` | Build timestamp                 |

Example:

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t scrq:1.0.0 .
```

## Docker Compose

### Basic Setup

```yaml
version: "3.8"
services:
  scrq:
    build: .
    ports:
      - "8000:8000"
    volumes:
      - scrq-data:/app/data
    environment:
      - SCRQ_HOST=0.0.0.0
      - SCRQ_PORT=8000
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8000/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  scrq-data:
```

### With Chrome

```yaml
version: "3.8"
services:
  scrq:
    build: .
    ports:
      - "8000:8000"
    command: ["--with-chrome"]
    shm_size: "2gb" # Required for Chrome
    volumes:
      - scrq-data:/app/data

volumes:
  scrq-data:
```

## Volume Mounts

| Path             | Description                        |
| ---------------- | ---------------------------------- |
| `/app/data/nats` | NATS JetStream persistence         |
| `/app/bin`       | Downloaded binaries (NATS, Chrome) |
| `/app/browser`   | Lightpanda browser binary          |

## Health Check

The container includes a built-in health check:

```bash
wget --spider -q http://localhost:8000/health
```

## Resource Limits

Recommended resource limits:

```yaml
services:
  scrq:
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: "2"
        reservations:
          memory: 512M
          cpus: "0.5"
```

## Security

### Running as Non-Root

The container runs as a non-root user by default (in production images).

### Network Security

- Only expose port 8000 externally
- Use reverse proxy (nginx, traefik) for TLS
- Consider network isolation for internal services
