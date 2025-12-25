# Scrq Security Features

Scrq implements several security features to protect against abuse and ensure reliable operation.

## Overview

| Feature            | Description                              | Default        |
| ------------------ | ---------------------------------------- | -------------- |
| Rate Limiting      | Limits requests per IP/user              | 100 req/min    |
| Idempotency        | Prevents duplicate job creation          | 24h TTL        |
| Job Timeout        | Maximum job execution time               | 30s (max 5min) |
| Retry with Backoff | Automatic retry with exponential backoff | 3 retries      |
| Result TTL         | Auto-cleanup of old results              | 7 days         |
| Security Headers   | Standard security HTTP headers           | Enabled        |

## Rate Limiting

Rate limiting uses a sliding window algorithm to limit requests per IP address.

### Configuration

```bash
./server --rate-limit 100  # 100 requests per minute
```

### Response Headers

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1704067200
```

### Rate Limit Exceeded

```json
HTTP/1.1 429 Too Many Requests
{
  "success": false,
  "error": "Rate limit exceeded. Try again in 45 seconds"
}
```

## Idempotency

Prevent duplicate job creation using idempotency keys.

### Usage

Include an idempotency key in your request header or body:

```bash
# Via header
curl -X POST http://localhost:8000/scrq/jobs \
  -H "X-Idempotency-Key: unique-request-id-12345" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'

# Via body
curl -X POST http://localhost:8000/scrq/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com",
    "idempotency_key": "unique-request-id-12345"
  }'
```

### Response

If a duplicate request is detected:

```json
HTTP/1.1 202 Accepted
X-Idempotency-Hit: true
{
  "success": true,
  "data": {
    "job_id": "original-job-id",
    "status": "queued",
    ...
  }
}
```

### Best Practices

1. Use UUID v4 for idempotency keys
2. Store the key with your request on the client side
3. Keys expire after 24 hours

## Job Timeout & Retry

### Timeout Configuration

```json
{
  "url": "https://example.com",
  "timeout": 60, // seconds (max 300)
  "max_retries": 3 // max 5
}
```

### Retry Behavior

| Retry | Delay      |
| ----- | ---------- |
| 1st   | 1 second   |
| 2nd   | 2 seconds  |
| 3rd   | 4 seconds  |
| 4th   | 8 seconds  |
| 5th   | 16 seconds |

### Status Response with Retry Info

```json
{
  "success": true,
  "data": {
    "job_id": "abc123",
    "status": "retrying",
    "retry_info": {
      "retry_count": 2,
      "max_retries": 3,
      "last_error": "timeout after 30s"
    },
    "next_retry_at": "2025-01-01T12:00:05Z"
  }
}
```

## Progress Updates

Jobs report detailed progress including page/item counts.

### Progress Response

```json
{
  "job_id": "abc123",
  "status": "running",
  "progress": 50,
  "message": "[Page 1/2] Fetching page",
  "progress_info": {
    "current_page": 1,
    "total_pages": 2,
    "current_item": 5,
    "total_items": 10,
    "stage": "fetching"
  }
}
```

### SSE Progress Events

```
data: {"job_id":"abc123","status":"running","progress":30,"message":"[Page 1/3] Processing"}

data: {"job_id":"abc123","status":"running","progress":60,"message":"[Page 2/3] Processing"}

data: {"job_id":"abc123","status":"succeeded","progress":100,"message":"Completed"}
```

## Result TTL

Job results are automatically cleaned up after 7 days.

### Status Response with Expiry

```json
{
  "job_id": "abc123",
  "status": "succeeded",
  "expires_at": "2025-01-08T12:00:00Z"
}
```

### Notes

- Results are removed from memory after TTL expires
- Cleanup runs every hour
- Accessing a job after expiry returns 404

## Security Headers

All responses include security headers:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
```

## IP Whitelist (Optional)

For additional security, you can whitelist specific IPs:

```go
// In your code
middleware := security.IPWhitelistMiddleware([]string{
    "127.0.0.1",
    "10.0.0.0/8",
    "192.168.1.0/24",
})
app.Use(middleware)
```

## Priority Queuing

Jobs can be assigned priority levels (1-10):

```json
{
  "url": "https://example.com",
  "priority": 8 // Higher = more priority
}
```

Default priority is 5.

## Best Practices

1. **Always use idempotency keys** for critical operations
2. **Set appropriate timeouts** based on expected page complexity
3. **Monitor rate limit headers** to avoid hitting limits
4. **Use webhooks** for long-running jobs instead of polling
5. **Handle retry scenarios** in your application logic
6. **Clean up completed jobs** that you no longer need

## Error Codes

| Code | Description                      |
| ---- | -------------------------------- |
| 400  | Bad request (invalid parameters) |
| 404  | Job not found or expired         |
| 409  | Job not yet completed            |
| 429  | Rate limit exceeded              |
| 500  | Internal server error            |

## Monitoring

Check rate limit status:

```bash
curl -I http://localhost:8000/scrq/jobs
```

Response headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Reset: 1704067260
```
