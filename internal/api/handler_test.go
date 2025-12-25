package api_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/go-rod-fiber-lightpanda-starter/internal/api"
	"github.com/gofiber/fiber/v2"
)

func setupTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: api.ErrorHandler,
	})

	// Mock handler without real browser
	handler := &MockHandler{}

	app.Get("/health", handler.HealthCheck)
	app.Get("/api/v1/browser/status", handler.BrowserStatus)
	app.Post("/api/v1/page/fetch", handler.FetchPage)
	app.Post("/api/v1/page/screenshot", handler.Screenshot)

	return app
}

// MockHandler is a mock handler for testing
type MockHandler struct{}

func (h *MockHandler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(api.Response{
		Success: true,
		Data: map[string]interface{}{
			"status": "ok",
		},
	})
}

func (h *MockHandler) BrowserStatus(c *fiber.Ctx) error {
	return c.JSON(api.Response{
		Success: true,
		Data: map[string]interface{}{
			"running":  true,
			"endpoint": "ws://127.0.0.1:9222",
		},
	})
}

func (h *MockHandler) FetchPage(c *fiber.Ctx) error {
	var req api.FetchRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	return c.JSON(api.Response{
		Success: true,
		Data: map[string]interface{}{
			"url":   req.URL,
			"title": "Test Page",
			"html":  "<html><body>Test</body></html>",
		},
	})
}

func (h *MockHandler) Screenshot(c *fiber.Ctx) error {
	var req api.ScreenshotRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	return c.JSON(api.Response{
		Success: true,
		Data: map[string]interface{}{
			"screenshot": "base64encodeddata",
			"format":     "png",
		},
	})
}

func TestHealthCheck(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success to be true")
	}
}

func TestBrowserStatus(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/api/v1/browser/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success to be true")
	}

	data := response.Data.(map[string]interface{})
	if data["running"] != true {
		t.Errorf("Expected browser to be running")
	}
}

func TestFetchPage(t *testing.T) {
	app := setupTestApp()

	// Test with valid URL
	reqBody := `{"url": "https://example.com"}`
	req := httptest.NewRequest("POST", "/api/v1/page/fetch", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success to be true")
	}
}

func TestFetchPageMissingURL(t *testing.T) {
	app := setupTestApp()

	// Test without URL
	reqBody := `{}`
	req := httptest.NewRequest("POST", "/api/v1/page/fetch", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestScreenshot(t *testing.T) {
	app := setupTestApp()

	reqBody := `{"url": "https://example.com", "full_page": true}`
	req := httptest.NewRequest("POST", "/api/v1/page/screenshot", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response api.Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success to be true")
	}

	data := response.Data.(map[string]interface{})
	if data["format"] != "png" {
		t.Errorf("Expected format to be png")
	}
}

func TestInvalidJSON(t *testing.T) {
	app := setupTestApp()

	reqBody := `{invalid json}`
	req := httptest.NewRequest("POST", "/api/v1/page/fetch", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}
