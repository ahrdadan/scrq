package api

import (
	"context"
	"encoding/base64"
	"sync"
	"time"

	"github.com/example/go-rod-fiber-lightpanda-starter/internal/browser"
	"github.com/gofiber/fiber/v2"
)

// Handler handles API requests
type Handler struct {
	browserManager browser.Client
}

// NewHandler creates a new handler
func NewHandler(browserManager browser.Client) *Handler {
	return &Handler{
		browserManager: browserManager,
	}
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ErrorHandler is the custom error handler for Fiber
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(Response{
		Success: false,
		Error:   err.Error(),
	})
}

// HealthCheck returns health status
func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// BrowserStatus returns browser status
func (h *Handler) BrowserStatus(c *fiber.Ctx) error {
	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"running":  h.browserManager.IsRunning(),
			"endpoint": h.browserManager.GetEndpoint(),
		},
	})
}

// RequestOptions represents optional browser settings for a request.
type RequestOptions struct {
	Timeout     int                   `json:"timeout"`
	WaitForLoad *bool                 `json:"wait_for_load,omitempty"`
	UserAgent   string                `json:"user_agent,omitempty"`
	Headers     map[string]string     `json:"headers,omitempty"`
	Cookies     []browser.CookieParam `json:"cookies,omitempty"`
	Proxy       string                `json:"proxy,omitempty"`
}

func buildPageOptions(req RequestOptions, defaultWait bool) browser.PageOptions {
	opts := browser.DefaultPageOptions()
	if req.Timeout > 0 {
		opts.Timeout = time.Duration(req.Timeout) * time.Second
	}
	if req.WaitForLoad != nil {
		opts.WaitForLoad = *req.WaitForLoad
	} else {
		opts.WaitForLoad = defaultWait
	}
	opts.UserAgent = req.UserAgent
	opts.Headers = req.Headers
	opts.Cookies = req.Cookies
	opts.Proxy = req.Proxy
	return opts
}

// FetchRequest represents a fetch request
type FetchRequest struct {
	URL        string `json:"url" validate:"required"`
	Screenshot bool   `json:"screenshot"`
	RequestOptions
}

// FetchPage fetches a page and returns its content
func (h *Handler) FetchPage(c *fiber.Ctx) error {
	var req FetchRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	opts := buildPageOptions(req.RequestOptions, false)
	opts.Screenshot = req.Screenshot

	ctx := context.Background()
	result, err := h.browserManager.FetchPage(ctx, req.URL, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Convert screenshot to base64 if present
	response := map[string]interface{}{
		"url":   result.URL,
		"title": result.Title,
		"html":  result.HTML,
		"text":  result.Text,
		"links": result.Links,
	}

	if len(result.Screenshot) > 0 {
		response["screenshot"] = base64.StdEncoding.EncodeToString(result.Screenshot)
	}

	return c.JSON(Response{
		Success: true,
		Data:    response,
	})
}

// ScreenshotRequest represents a screenshot request
type ScreenshotRequest struct {
	URL      string `json:"url" validate:"required"`
	FullPage bool   `json:"full_page"`
	RequestOptions
}

// Screenshot takes a screenshot of a page
func (h *Handler) Screenshot(c *fiber.Ctx) error {
	var req ScreenshotRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, true)
	screenshot, err := h.browserManager.TakeScreenshot(ctx, req.URL, req.FullPage, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"screenshot": base64.StdEncoding.EncodeToString(screenshot),
			"format":     "png",
		},
	})
}

// EvaluateRequest represents a script evaluation request
type EvaluateRequest struct {
	URL    string `json:"url" validate:"required"`
	Script string `json:"script" validate:"required"`
	RequestOptions
}

// EvaluateScript evaluates JavaScript on a page
func (h *Handler) EvaluateScript(c *fiber.Ctx) error {
	var req EvaluateRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" || req.Script == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL and script are required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, true)
	result, err := h.browserManager.EvaluateScript(ctx, req.URL, req.Script, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"result": result,
		},
	})
}

// ClickRequest represents a click request
type ClickRequest struct {
	URL      string `json:"url" validate:"required"`
	Selector string `json:"selector" validate:"required"`
	RequestOptions
}

// ClickElement clicks an element on a page
func (h *Handler) ClickElement(c *fiber.Ctx) error {
	var req ClickRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" || req.Selector == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL and selector are required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, true)
	err := h.browserManager.ClickElement(ctx, req.URL, req.Selector, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"clicked": true,
		},
	})
}

// FillRequest represents a form fill request
type FillRequest struct {
	URL    string            `json:"url" validate:"required"`
	Inputs map[string]string `json:"inputs" validate:"required"`
	RequestOptions
}

// FillForm fills form inputs on a page
func (h *Handler) FillForm(c *fiber.Ctx) error {
	var req FillRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" || len(req.Inputs) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "URL and inputs are required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, true)
	err := h.browserManager.FillForm(ctx, req.URL, req.Inputs, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"filled": true,
		},
	})
}

// LinksRequest represents a links extraction request
type LinksRequest struct {
	URL string `json:"url" validate:"required"`
	RequestOptions
}

// ExtractLinks extracts all links from a page
func (h *Handler) ExtractLinks(c *fiber.Ctx) error {
	var req LinksRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	opts := buildPageOptions(req.RequestOptions, false)
	ctx := context.Background()
	result, err := h.browserManager.FetchPage(ctx, req.URL, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"url":   result.URL,
			"links": result.Links,
			"count": len(result.Links),
		},
	})
}

// GetPageInfo returns basic page information
func (h *Handler) GetPageInfo(c *fiber.Ctx) error {
	var req LinksRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, true)
	result, err := h.browserManager.GetPageInfo(ctx, req.URL, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data:    result,
	})
}

// ScrapeRequest represents a scraping request
type ScrapeRequest struct {
	URL       string   `json:"url" validate:"required"`
	Selectors []string `json:"selectors"`
	Script    string   `json:"script"`
	RequestOptions
}

// Scrape scrapes data from a page
func (h *Handler) Scrape(c *fiber.Ctx) error {
	var req ScrapeRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	ctx := context.Background()
	opts := buildPageOptions(req.RequestOptions, req.Script != "")

	// If custom script provided, use it
	if req.Script != "" {
		result, err := h.browserManager.EvaluateScript(ctx, req.URL, req.Script, opts)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		return c.JSON(Response{
			Success: true,
			Data: map[string]interface{}{
				"url":    req.URL,
				"result": result,
			},
		})
	}

	// Otherwise fetch page content
	result, err := h.browserManager.FetchPage(ctx, req.URL, opts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"url":   result.URL,
			"title": result.Title,
			"html":  result.HTML,
			"text":  result.Text,
		},
	})
}

// BatchScrapeRequest represents a batch scraping request
type BatchScrapeRequest struct {
	URLs       []string `json:"urls" validate:"required"`
	Script     string   `json:"script"`
	Concurrent int      `json:"concurrent"`
	RequestOptions
}

// BatchScrapeResult represents a single result in batch scraping
type BatchScrapeResult struct {
	URL   string      `json:"url"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// BatchScrape scrapes multiple pages concurrently
func (h *Handler) BatchScrape(c *fiber.Ctx) error {
	var req BatchScrapeRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if len(req.URLs) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "URLs are required")
	}

	concurrent := req.Concurrent
	if concurrent <= 0 {
		concurrent = 3
	}
	if concurrent > 10 {
		concurrent = 10
	}

	results := make([]BatchScrapeResult, len(req.URLs))
	opts := buildPageOptions(req.RequestOptions, req.Script != "")
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrent)

	for i, url := range req.URLs {
		wg.Add(1)
		go func(idx int, targetURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ctx := context.Background()
			result := BatchScrapeResult{URL: targetURL}

			if req.Script != "" {
				data, err := h.browserManager.EvaluateScript(ctx, targetURL, req.Script, opts)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.Data = data
				}
			} else {
				pageResult, err := h.browserManager.FetchPage(ctx, targetURL, opts)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.Data = map[string]interface{}{
						"title": pageResult.Title,
						"text":  pageResult.Text,
						"links": pageResult.Links,
					}
				}
			}

			results[idx] = result
		}(i, url)
	}

	wg.Wait()

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"results": results,
			"total":   len(results),
		},
	})
}
