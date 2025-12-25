package security

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Middleware provides security middleware for Fiber
type Middleware struct {
	rateLimiter      *RateLimiter
	idempotencyStore *IdempotencyStore
}

// NewMiddleware creates a new security middleware
func NewMiddleware(rl *RateLimiter, is *IdempotencyStore) *Middleware {
	return &Middleware{
		rateLimiter:      rl,
		idempotencyStore: is,
	}
}

// RateLimitMiddleware returns a rate limiting middleware
func (m *Middleware) RateLimitMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get client identifier (prefer user ID, fallback to IP)
		clientID := c.Get("X-User-ID")
		if clientID == "" {
			clientID = c.Get("X-API-Key")
		}
		if clientID == "" {
			clientID = c.IP()
		}

		// Check rate limit
		if !m.rateLimiter.Allow(clientID) {
			info := m.rateLimiter.GetInfo(clientID)

			c.Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
			c.Set("X-RateLimit-Remaining", "0")
			c.Set("X-RateLimit-Reset", strconv.FormatInt(info.ResetAt.Unix(), 10))
			c.Set("Retry-After", strconv.FormatInt(int64(time.Until(info.ResetAt).Seconds()), 10))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success":     false,
				"error":       "Rate limit exceeded",
				"retry_after": int64(time.Until(info.ResetAt).Seconds()),
			})
		}

		// Add rate limit headers
		info := m.rateLimiter.GetInfo(clientID)
		c.Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(info.ResetAt.Unix(), 10))

		return c.Next()
	}
}

// IdempotencyMiddleware returns an idempotency middleware for POST requests
func (m *Middleware) IdempotencyMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only apply to POST requests
		if c.Method() != fiber.MethodPost {
			return c.Next()
		}

		// Check for idempotency key header
		idempotencyKey := c.Get("X-Idempotency-Key")
		if idempotencyKey == "" {
			return c.Next()
		}

		// Check if we have a cached response
		entry, exists := m.idempotencyStore.Check(idempotencyKey)
		if exists {
			c.Set("X-Idempotency-Replayed", "true")
			return c.Status(fiber.StatusAccepted).JSON(entry.Response)
		}

		// Continue with the request
		return c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'self'")

		// Generate request ID if not present
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = GenerateRequestID()
		}
		c.Set("X-Request-ID", requestID)
		c.Locals("requestID", requestID)

		return c.Next()
	}
}

// RequestValidationMiddleware validates incoming requests
func RequestValidationMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check content type for POST/PUT/PATCH requests
		if c.Method() == fiber.MethodPost || c.Method() == fiber.MethodPut || c.Method() == fiber.MethodPatch {
			contentType := c.Get("Content-Type")
			if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
				return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{
					"success": false,
					"error":   "Content-Type must be application/json",
				})
			}
		}

		// Limit request body size (10MB max)
		if len(c.Body()) > 10*1024*1024 {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"success": false,
				"error":   "Request body too large",
			})
		}

		return c.Next()
	}
}

// IPWhitelistMiddleware creates an IP whitelist middleware
func IPWhitelistMiddleware(allowedIPs []string) fiber.Handler {
	ipSet := make(map[string]bool)
	for _, ip := range allowedIPs {
		ipSet[ip] = true
	}

	return func(c *fiber.Ctx) error {
		if len(allowedIPs) == 0 {
			return c.Next()
		}

		clientIP := c.IP()
		if !ipSet[clientIP] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Access denied",
			})
		}

		return c.Next()
	}
}
