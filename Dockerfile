# Build stage
FROM golang:1.24-alpine AS builder

# Install git for version info
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=1
ARG BUILD_TIME=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X github.com/example/go-rod-fiber-lightpanda-starter/internal/config.Version=${VERSION}" \
    -o server ./cmd/server

# Runtime stage
FROM debian:bookworm-slim

# Build arguments for labels
ARG VERSION=1

# Labels
LABEL org.opencontainers.image.title="Scrq"
LABEL org.opencontainers.image.description="Asynchronous web scraping API"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.source="https://github.com/ahrdadan/scrq"
LABEL org.opencontainers.image.licenses="MIT"

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Create directories
RUN mkdir -p /app/browser /app/bin /app/data/nats

# Copy the binary from builder
COPY --from=builder /app/server .

# Download Lightpanda browser (auto-downloaded at runtime if not present)
# Comment out this line if you want automatic download at startup
RUN wget -q -O ./browser/lightpanda-x86_64-linux \
    https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-x86_64-linux && \
    chmod +x ./browser/lightpanda-x86_64-linux || true

# Make server executable
RUN chmod +x ./server

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# Run the server
ENTRYPOINT ["./server"]
CMD ["--host", "0.0.0.0", "--port", "8000"]
