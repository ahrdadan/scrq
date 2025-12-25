# Scrq Architecture

## Overview

Scrq is designed as an asynchronous web scraping system with queue-based job processing.

```
┌─────────────────────────────────────────────────────────────┐
│                        Client                                │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    Fiber HTTP Server                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   /health   │  │   /scrq/*   │  │   /scrq/ws          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────┬───────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │ Job Queue  │  │  Sync API  │  │  WebSocket │
   │  Manager   │  │  Handler   │  │   Handler  │
   └─────┬──────┘  └─────┬──────┘  └────────────┘
         │               │
         ▼               ▼
   ┌────────────┐  ┌────────────────────────────┐
   │   NATS     │  │      Browser Clients       │
   │ JetStream  │  │  ┌──────────┐ ┌─────────┐  │
   └─────┬──────┘  │  │Lightpanda│ │ Chrome  │  │
         │         │  └──────────┘ └─────────┘  │
         ▼         └────────────────────────────┘
   ┌────────────┐
   │   Worker   │
   │ (Processor)│
   └────────────┘
```

## Components

### HTTP Server (Fiber)

- Fast, low-memory footprint
- Middleware: CORS, Logger, Recover
- Routes: `/health`, `/scrq/*`

### Job Queue Manager

- Uses NATS JetStream for persistence
- In-memory job store for quick lookups
- Event hub for real-time notifications

### Browser Clients

#### Lightpanda

- Lightweight headless browser
- Linux only
- Auto-download from GitHub releases
- CDP protocol via Rod

#### Chrome

- Full Chromium via Rod launcher
- Supports proxy
- Auto-download via Rod

### NATS JetStream

- Portable (auto-download binary)
- Persistent job queue
- Work queue semantics
- At-least-once delivery

## Job Flow

```
1. Client POST /scrq/jobs
   └─> Create Job (status: queued)
   └─> Save to Store
   └─> Publish to NATS

2. Worker fetches from NATS
   └─> Update status: running
   └─> Process with browser
   └─> Update progress via events

3. Job completes
   └─> Update status: succeeded/failed
   └─> Save result
   └─> Send webhook (if configured)
   └─> Emit final event
```

## Event System

### Server-Sent Events (SSE)

- `GET /scrq/jobs/{id}/events`
- Automatic reconnection
- Suitable for browsers

### WebSocket

- `GET /scrq/ws?job_id={id}`
- Bidirectional (future: job control)
- Lower latency

### Webhooks

- POST to user-defined URL
- Includes job status and result URL
- Optional HMAC signature

## Data Persistence

### Job Store

- In-memory map (fast lookups)
- NATS JetStream (durability)
- 24-hour retention

### NATS Store

- File-based storage
- Configurable directory
- Survives restarts

## Error Handling

1. **Browser errors**: Retried up to 3 times
2. **Network errors**: Exponential backoff
3. **Job failures**: Marked as failed, notified

## Scalability

Current design is single-node. Future considerations:

- Multiple workers (NATS consumer groups)
- External NATS cluster
- Redis for distributed job store
- Horizontal pod autoscaling (K8s)
