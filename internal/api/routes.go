package api

import (
	"time"

	"github.com/ahrdadan/scrq/internal/browser"
	"github.com/ahrdadan/scrq/internal/queue"
	"github.com/ahrdadan/scrq/internal/security"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// SetupRoutes configures all API routes
func SetupRoutes(app *fiber.App, browserManager browser.Client) {
	handler := NewHandler(browserManager)

	// Health check (simple path)
	app.Get("/health", handler.HealthCheck)

	// Scrq routes
	registerRoutes(app.Group("/scrq"), handler)
}

// SetupChromeRoutes registers routes that use the Chrome backend.
func SetupChromeRoutes(app *fiber.App, chromeManager browser.Client) {
	handler := NewHandler(chromeManager)
	registerRoutes(app.Group("/scrq/chrome"), handler)
}

// RouteConfig holds configuration for routes
type RouteConfig struct {
	RateLimitRequests int           // requests per window
	RateLimitWindow   time.Duration // time window
	IdempotencyTTL    time.Duration // TTL for idempotency keys
	BaseURL           string        // Base URL for full URLs in responses
}

// DefaultRouteConfig returns default route configuration
func DefaultRouteConfig() RouteConfig {
	return RouteConfig{
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
		IdempotencyTTL:    24 * time.Hour,
		BaseURL:           "http://localhost:8000",
	}
}

// SetupJobRoutes configures job queue routes
func SetupJobRoutes(app *fiber.App, queueManager *queue.Manager) {
	SetupJobRoutesWithConfig(app, queueManager, DefaultRouteConfig())
}

// SetupJobRoutesWithConfig configures job queue routes with custom config
func SetupJobRoutesWithConfig(app *fiber.App, queueManager *queue.Manager, config RouteConfig) {
	// Create security stores
	rateLimiter := security.NewRateLimiter(security.RateLimitConfig{
		RequestsPerWindow: config.RateLimitRequests,
		WindowDuration:    config.RateLimitWindow,
		BurstMax:          20,
	})
	idempotencyStore := security.NewIdempotencyStore(config.IdempotencyTTL)

	jobHandler := NewJobHandlerWithConfig(queueManager, idempotencyStore, config.BaseURL)

	// Create security middleware
	secMiddleware := security.NewMiddleware(rateLimiter, idempotencyStore)

	scrq := app.Group("/scrq")

	// Apply security headers to all scrq routes
	scrq.Use(security.SecurityHeadersMiddleware())

	// Job queue endpoints with rate limiting
	jobsGroup := scrq.Group("/jobs")
	jobsGroup.Use(secMiddleware.RateLimitMiddleware())

	jobsGroup.Post("", jobHandler.CreateJob)
	jobsGroup.Get("/:job_id", jobHandler.GetJobStatus)
	jobsGroup.Get("/:job_id/result", jobHandler.GetJobResult)
	jobsGroup.Post("/:job_id/cancel", jobHandler.CancelJob)
	jobsGroup.Get("/:job_id/events", jobHandler.StreamEvents)

	// WebSocket endpoint for job events
	app.Use("/scrq/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/scrq/ws", websocket.New(jobHandler.HandleWebSocket))
}

// SetupSecureRoutes configures routes with full security middleware
func SetupSecureRoutes(app *fiber.App, browserManager browser.Client, config RouteConfig) {
	handler := NewHandler(browserManager)

	// Create rate limiter
	rateLimiter := security.NewRateLimiter(security.RateLimitConfig{
		RequestsPerWindow: config.RateLimitRequests,
		WindowDuration:    config.RateLimitWindow,
		BurstMax:          20,
	})

	// Create security middleware
	secMiddleware := security.NewMiddleware(rateLimiter, nil)

	// Health check (no rate limit)
	app.Get("/health", handler.HealthCheck)

	// Scrq routes with security
	scrq := app.Group("/scrq")
	scrq.Use(security.SecurityHeadersMiddleware())
	scrq.Use(secMiddleware.RateLimitMiddleware())

	registerRoutes(scrq, handler)
}

func registerRoutes(scrq fiber.Router, handler *Handler) {
	// Browser status
	scrq.Get("/browser/status", handler.BrowserStatus)

	// Page operations
	scrq.Post("/page/fetch", handler.FetchPage)
	scrq.Post("/page/screenshot", handler.Screenshot)
	scrq.Post("/page/evaluate", handler.EvaluateScript)
	scrq.Post("/page/click", handler.ClickElement)
	scrq.Post("/page/fill", handler.FillForm)
	scrq.Post("/page/links", handler.ExtractLinks)
	scrq.Post("/page/info", handler.GetPageInfo)

	// Scraping operations
	scrq.Post("/scrape", handler.Scrape)
	scrq.Post("/scrape/batch", handler.BatchScrape)
}
