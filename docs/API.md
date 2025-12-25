# Scrq API Documentation

## Overview

Scrq (Scrape + Queue) is an asynchronous web scraping API built with Go + Fiber + Rod + Lightpanda. It's designed for long-running scraping tasks with queue-based processing.

## Base URL

All API endpoints are prefixed with `/scrq`.

## Response Format

All responses follow this format:

```json
{
  "success": true,
  "data": {},
  "error": "error message (only when success is false)"
}
```

## Endpoints

### Health Check

#### `GET /health`

Returns the server health status.

**Response:**

```json
{
  "success": true,
  "data": {
    "status": "ok",
    "timestamp": "2025-01-01T12:00:00Z"
  }
}
```

### Browser Status

#### `GET /scrq/browser/status`

Returns the browser status and WebSocket endpoint.

**Response:**

```json
{
  "success": true,
  "data": {
    "running": true,
    "endpoint": "ws://127.0.0.1:9222"
  }
}
```

### Async Job Queue

#### `POST /scrq/jobs` - Create Job

Creates a new asynchronous scraping job.

**Request:**

```json
{
  "type": "scrape",
  "url": "https://example.com",
  "engine": "lightpanda",
  "timeout": 30,
  "wait_for_load": true,
  "script": "() => ({ title: document.title })",
  "user_agent": "Mozilla/5.0 ...",
  "headers": { "Accept-Language": "en-US,en;q=0.9" },
  "cookies": [{ "name": "session", "value": "abc", "domain": "example.com" }],
  "proxy": "http://user:pass@proxy-host:8080",
  "notify": {
    "webhook_url": "https://yourapp.com/webhooks/scrq",
    "websocket": true
  }
}
```

| Field         | Type   | Description                                        |
| ------------- | ------ | -------------------------------------------------- |
| type          | string | Job type. Currently only `scrape` is supported     |
| url           | string | **Required.** URL to scrape                        |
| engine        | string | Browser engine: `lightpanda` (default) or `chrome` |
| timeout       | int    | Timeout in seconds (default: 30)                   |
| wait_for_load | bool   | Wait for page load (default: true)                 |
| script        | string | JavaScript to execute on the page                  |
| user_agent    | string | Custom User-Agent header                           |
| headers       | object | Custom HTTP headers                                |
| cookies       | array  | Cookies to set                                     |
| proxy         | string | Proxy URL (chrome engine only)                     |
| notify        | object | Notification settings                              |

**Response (202 Accepted):**

```json
{
  "success": true,
  "data": {
    "job_id": "job_123abc",
    "status": "queued",
    "status_url": "/scrq/jobs/job_123abc",
    "result_url": "/scrq/jobs/job_123abc/result",
    "events": {
      "sse_url": "/scrq/jobs/job_123abc/events",
      "ws_url": "/scrq/ws?job_id=job_123abc"
    }
  }
}
```

#### `GET /scrq/jobs/{job_id}` - Get Job Status

Returns the current status of a job.

**Response:**

```json
{
  "success": true,
  "data": {
    "job_id": "job_123abc",
    "status": "running",
    "progress": 35,
    "message": "Fetching page",
    "created_at": 1710000000,
    "updated_at": 1710000123
  }
}
```

**Status values:**

- `queued` - Job is waiting to be processed
- `running` - Job is currently being processed
- `succeeded` - Job completed successfully
- `failed` - Job failed
- `canceled` - Job was canceled

#### `GET /scrq/jobs/{job_id}/result` - Get Job Result

Returns the result of a completed job.

**Response (200 OK):**

```json
{
  "success": true,
  "data": {
    "job_id": "job_123abc",
    "status": "succeeded",
    "result": {
      "title": "Example Domain",
      "links": ["https://..."]
    }
  }
}
```

**Response (409 Conflict):** When job is not completed yet.

#### `POST /scrq/jobs/{job_id}/cancel` - Cancel Job

Cancels a queued or running job.

**Response:**

```json
{
  "success": true,
  "data": {
    "job_id": "job_123abc",
    "status": "canceled"
  }
}
```

#### `GET /scrq/jobs/{job_id}/events` - Stream Events (SSE)

Server-Sent Events stream for real-time job updates.

**Event format:**

```
data: {"job_id":"job_123abc","status":"running","progress":35,"message":"..."}
```

### WebSocket

#### `GET /scrq/ws?job_id={job_id}`

WebSocket connection for real-time job events.

**Events:**

- `job.queued`
- `job.running`
- `job.progress`
- `job.succeeded`
- `job.failed`

### Synchronous Endpoints

These endpoints are for quick, synchronous operations:

#### `POST /scrq/page/fetch`

Fetches a page and returns its content.

#### `POST /scrq/page/screenshot`

Takes a screenshot of a page.

#### `POST /scrq/page/evaluate`

Evaluates JavaScript on a page.

#### `POST /scrq/page/click`

Clicks an element on a page.

#### `POST /scrq/page/fill`

Fills form inputs on a page.

#### `POST /scrq/page/links`

Extracts links from a page.

#### `POST /scrq/page/info`

Gets basic page information.

#### `POST /scrq/scrape`

Scrapes data from a page.

#### `POST /scrq/scrape/batch`

Scrapes multiple pages concurrently.

### Chrome Endpoints

Chrome-backed endpoints are available at `/scrq/chrome/*` when Chrome is enabled. These support proxy configuration.

## Webhook Notifications

When `notify.webhook_url` is provided, Scrq sends a POST request when the job completes:

```json
{
  "job_id": "job_123abc",
  "status": "succeeded",
  "result_url": "https://your-scrq-host/scrq/jobs/job_123abc/result",
  "finished_at": 1710000999
}
```

Headers:

- `Content-Type: application/json`
- `X-Scrq-Event: job.succeeded`
