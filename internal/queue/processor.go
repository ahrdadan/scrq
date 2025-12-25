package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/example/go-rod-fiber-lightpanda-starter/internal/browser"
)

// ScrapeProcessor processes scrape jobs
type ScrapeProcessor struct {
	lightpanda browser.Client
	chrome     browser.Client
}

// NewScrapeProcessor creates a new scrape processor
func NewScrapeProcessor(lightpanda, chrome browser.Client) *ScrapeProcessor {
	return &ScrapeProcessor{
		lightpanda: lightpanda,
		chrome:     chrome,
	}
}

// ProgressReporter provides methods for reporting detailed progress
type ProgressReporter struct {
	job          *Job
	updateFunc   func(int, string)
	currentStage string
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(job *Job, updateFunc func(int, string)) *ProgressReporter {
	return &ProgressReporter{
		job:        job,
		updateFunc: updateFunc,
	}
}

// SetStage sets the current processing stage
func (r *ProgressReporter) SetStage(stage string) {
	r.currentStage = stage
	if r.job.ProgressInfo == nil {
		r.job.ProgressInfo = &ProgressInfo{}
	}
	r.job.ProgressInfo.Stage = stage
}

// SetPageProgress sets page progress (page X of Y)
func (r *ProgressReporter) SetPageProgress(current, total int, message string) {
	if r.job.ProgressInfo == nil {
		r.job.ProgressInfo = &ProgressInfo{}
	}
	r.job.ProgressInfo.CurrentPage = current
	r.job.ProgressInfo.TotalPages = total

	// Calculate percentage
	var pct int
	if total > 0 {
		pct = (current * 100) / total
	}

	fullMessage := fmt.Sprintf("[Page %d/%d] %s", current, total, message)
	r.updateFunc(pct, fullMessage)
}

// SetItemProgress sets item progress (item X of Y)
func (r *ProgressReporter) SetItemProgress(current, total int, message string) {
	if r.job.ProgressInfo == nil {
		r.job.ProgressInfo = &ProgressInfo{}
	}
	r.job.ProgressInfo.CurrentItem = current
	r.job.ProgressInfo.TotalItems = total

	// Calculate percentage
	var pct int
	if total > 0 {
		pct = (current * 100) / total
	}

	fullMessage := fmt.Sprintf("[Item %d/%d] %s", current, total, message)
	r.updateFunc(pct, fullMessage)
}

// Report reports simple percentage progress
func (r *ProgressReporter) Report(pct int, message string) {
	r.updateFunc(pct, message)
}

// Process processes a scrape job
func (p *ScrapeProcessor) Process(ctx context.Context, job *Job, progress func(int, string)) (interface{}, error) {
	req := job.Request

	// Create progress reporter for detailed progress tracking
	reporter := NewProgressReporter(job, progress)
	reporter.SetStage("initialization")

	// Select browser client based on engine
	var client browser.Client
	switch req.Engine {
	case "chrome":
		if p.chrome == nil {
			return nil, fmt.Errorf("chrome engine not available")
		}
		client = p.chrome
	case "lightpanda", "":
		if p.lightpanda == nil {
			return nil, fmt.Errorf("lightpanda engine not available")
		}
		client = p.lightpanda
		if req.Proxy != "" {
			return nil, fmt.Errorf("proxy is only supported with chrome engine")
		}
	default:
		return nil, fmt.Errorf("unknown engine: %s", req.Engine)
	}

	reporter.Report(10, "Initializing browser")
	reporter.SetStage("browser_ready")

	// Build page options
	opts := browser.DefaultPageOptions()
	if req.Timeout > 0 {
		opts.Timeout = time.Duration(req.Timeout) * time.Second
	}
	opts.WaitForLoad = req.WaitForLoad
	opts.UserAgent = req.UserAgent
	opts.Headers = req.Headers
	opts.Proxy = req.Proxy

	// Convert cookies
	for _, c := range req.Cookies {
		opts.Cookies = append(opts.Cookies, browser.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			URL:      c.URL,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	reporter.SetStage("fetching")
	reporter.SetPageProgress(1, 1, "Fetching page")

	var result interface{}
	var err error

	// Check context before processing
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("job timed out: %w", ctx.Err())
	default:
	}

	if req.Script != "" {
		reporter.SetStage("script_execution")
		reporter.Report(50, "Executing script")
		result, err = client.EvaluateScript(ctx, req.URL, req.Script, opts)
	} else {
		result, err = client.FetchPage(ctx, req.URL, opts)
	}

	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() != nil {
			return nil, fmt.Errorf("job timed out after %v: %w", job.GetTimeoutDuration(), ctx.Err())
		}
		return nil, fmt.Errorf("scraping failed: %w", err)
	}

	reporter.SetStage("processing")
	reporter.Report(90, "Processing result")

	// Send webhook if configured
	if job.Notify != nil && job.Notify.WebhookURL != "" {
		go sendWebhook(job.ID, job.Notify.WebhookURL, "succeeded")
	}

	reporter.SetStage("completed")
	reporter.Report(100, "Job completed successfully")

	return result, nil
}

// sendWebhook sends a webhook notification
func sendWebhook(jobID, webhookURL, status string) {
	payload := map[string]interface{}{
		"job_id":      jobID,
		"status":      status,
		"result_url":  fmt.Sprintf("/scrq/jobs/%s/result", jobID),
		"finished_at": time.Now().Unix(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal webhook payload: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to create webhook request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scrq-Event", "job."+status)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to send webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Webhook returned error status: %d", resp.StatusCode)
	}
}
