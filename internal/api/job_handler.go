package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ahrdadan/scrq/internal/queue"
	"github.com/ahrdadan/scrq/internal/security"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// JobHandler handles job-related API requests
type JobHandler struct {
	queueManager     *queue.Manager
	idempotencyStore *security.IdempotencyStore
}

// NewJobHandler creates a new job handler
func NewJobHandler(qm *queue.Manager) *JobHandler {
	return &JobHandler{
		queueManager:     qm,
		idempotencyStore: security.NewIdempotencyStore(24 * time.Hour), // 24h TTL for idempotency keys
	}
}

// NewJobHandlerWithSecurity creates a new job handler with security store
func NewJobHandlerWithSecurity(qm *queue.Manager, idempotencyStore *security.IdempotencyStore) *JobHandler {
	return &JobHandler{
		queueManager:     qm,
		idempotencyStore: idempotencyStore,
	}
}

// CreateJobRequest extends JobRequest with security fields
type CreateJobRequest struct {
	queue.JobRequest
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Priority       int    `json:"priority,omitempty"` // 1-10, higher = more priority
	Timeout        int    `json:"timeout,omitempty"`  // seconds
	MaxRetries     int    `json:"max_retries,omitempty"`
}

// CreateJob creates a new async job
// POST /scrq/jobs
func (h *JobHandler) CreateJob(c *fiber.Ctx) error {
	var req CreateJobRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.JobRequest.URL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "URL is required")
	}

	if req.JobRequest.Type == "" {
		req.JobRequest.Type = queue.JobTypeScrape
	}

	// Check idempotency key from header or body
	idempotencyKey := c.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = req.IdempotencyKey
	}

	// If idempotency key provided, check for cached response
	if idempotencyKey != "" && h.idempotencyStore != nil {
		if cachedResponse, exists := h.idempotencyStore.Check(idempotencyKey); exists {
			c.Set("X-Idempotency-Hit", "true")
			return c.Status(fiber.StatusAccepted).JSON(Response{
				Success: true,
				Data:    cachedResponse,
			})
		}
	}

	job := queue.NewJob(req.JobRequest)

	// Set idempotency key
	if idempotencyKey != "" {
		job.IdempotencyKey = idempotencyKey
	}

	// Set priority (default 5)
	if req.Priority > 0 && req.Priority <= 10 {
		job.Priority = req.Priority
	} else {
		job.Priority = 5
	}

	// Set timeout (default 30s, max 5min)
	if req.Timeout > 0 {
		if req.Timeout > 300 {
			req.Timeout = 300
		}
		job.Timeout = req.Timeout
	}

	// Set max retries (default 3, max 5)
	if req.MaxRetries > 0 {
		if req.MaxRetries > 5 {
			req.MaxRetries = 5
		}
		job.MaxRetries = req.MaxRetries
	}

	// Enqueue with idempotency check
	enqueuedJob, wasDuplicate, err := h.queueManager.EnqueueWithIdempotency(job)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to enqueue job: %v", err))
	}

	response := queue.JobCreatedResponse{
		JobID:     enqueuedJob.ID,
		Status:    enqueuedJob.Status,
		StatusURL: fmt.Sprintf("/scrq/jobs/%s", enqueuedJob.ID),
		ResultURL: fmt.Sprintf("/scrq/jobs/%s/result", enqueuedJob.ID),
	}
	response.Events.SSEURL = fmt.Sprintf("/scrq/jobs/%s/events", enqueuedJob.ID)
	response.Events.WSURL = fmt.Sprintf("/scrq/ws?job_id=%s", enqueuedJob.ID)

	// Cache response for idempotency
	if idempotencyKey != "" && h.idempotencyStore != nil && !wasDuplicate {
		h.idempotencyStore.Store(idempotencyKey, enqueuedJob.ID, response)
	}

	if wasDuplicate {
		c.Set("X-Idempotency-Hit", "true")
	}

	return c.Status(fiber.StatusAccepted).JSON(Response{
		Success: true,
		Data:    response,
	})
}

// GetJobStatus returns the status of a job
// GET /scrq/jobs/:job_id
func (h *JobHandler) GetJobStatus(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	if jobID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Job ID is required")
	}

	job, err := h.queueManager.GetJob(jobID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Job not found")
	}

	response := map[string]interface{}{
		"job_id":     job.ID,
		"status":     job.Status,
		"progress":   job.Progress,
		"message":    job.Message,
		"created_at": job.CreatedAt,
		"updated_at": job.UpdatedAt,
		"priority":   job.Priority,
	}

	// Add progress info if available
	if job.ProgressInfo != nil {
		response["progress_info"] = map[string]interface{}{
			"current_page": job.ProgressInfo.CurrentPage,
			"total_pages":  job.ProgressInfo.TotalPages,
			"current_item": job.ProgressInfo.CurrentItem,
			"total_items":  job.ProgressInfo.TotalItems,
			"stage":        job.ProgressInfo.Stage,
		}
	}

	// Add retry info if retrying
	if job.Status == queue.JobStatusRetrying || job.RetryCount > 0 {
		response["retry_info"] = map[string]interface{}{
			"retry_count": job.RetryCount,
			"max_retries": job.MaxRetries,
			"last_error":  job.LastError,
		}
		if job.NextRetryAt > 0 {
			response["next_retry_at"] = time.Unix(job.NextRetryAt, 0).Format(time.RFC3339)
		}
	}

	// Add TTL info
	if job.ExpiresAt > 0 {
		response["expires_at"] = time.Unix(job.ExpiresAt, 0).Format(time.RFC3339)
	}

	return c.JSON(Response{
		Success: true,
		Data:    response,
	})
}

// GetJobResult returns the result of a completed job
// GET /scrq/jobs/:job_id/result
func (h *JobHandler) GetJobResult(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	if jobID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Job ID is required")
	}

	job, err := h.queueManager.GetJob(jobID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Job not found")
	}

	if job.Status != queue.JobStatusSucceeded && job.Status != queue.JobStatusFailed {
		return fiber.NewError(fiber.StatusConflict, "Job not completed yet")
	}

	return c.JSON(Response{
		Success: true,
		Data: queue.JobResultResponse{
			JobID:  job.ID,
			Status: job.Status,
			Result: job.Result,
			Error:  job.Error,
		},
	})
}

// CancelJob cancels a queued or running job
// POST /scrq/jobs/:job_id/cancel
func (h *JobHandler) CancelJob(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	if jobID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Job ID is required")
	}

	job, err := h.queueManager.CancelJob(jobID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(Response{
		Success: true,
		Data: map[string]interface{}{
			"job_id": job.ID,
			"status": job.Status,
		},
	})
}

// StreamEvents streams job events via SSE
// GET /scrq/jobs/:job_id/events
func (h *JobHandler) StreamEvents(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	if jobID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Job ID is required")
	}

	// Check if job exists
	job, err := h.queueManager.GetJob(jobID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Job not found")
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial status
		eventData, _ := json.Marshal(queue.Event{
			JobID:    job.ID,
			Status:   job.Status,
			Progress: job.Progress,
			Message:  job.Message,
		})
		fmt.Fprintf(w, "data: %s\n\n", eventData)
		w.Flush()

		// If job is already completed, close the stream
		if job.Status == queue.JobStatusSucceeded || job.Status == queue.JobStatusFailed || job.Status == queue.JobStatusCanceled {
			return
		}

		// Subscribe to events
		events := h.queueManager.Subscribe(jobID)
		defer h.queueManager.Unsubscribe(jobID, events)

		for event := range events {
			eventData, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			w.Flush()

			// Close stream when job completes
			if event.Status == queue.JobStatusSucceeded || event.Status == queue.JobStatusFailed || event.Status == queue.JobStatusCanceled {
				return
			}
		}
	})

	return nil
}

// HandleWebSocket handles WebSocket connections for job events
func (h *JobHandler) HandleWebSocket(c *websocket.Conn) {
	jobID := c.Query("job_id")
	if jobID == "" {
		_ = c.WriteJSON(map[string]interface{}{
			"error": "job_id is required",
		})
		c.Close()
		return
	}

	// Check if job exists
	job, err := h.queueManager.GetJob(jobID)
	if err != nil {
		_ = c.WriteJSON(map[string]interface{}{
			"error": "job not found",
		})
		c.Close()
		return
	}

	// Send initial status
	_ = c.WriteJSON(queue.Event{
		JobID:    job.ID,
		Status:   job.Status,
		Progress: job.Progress,
		Message:  job.Message,
	})

	// If job is already completed, close the connection
	if job.Status == queue.JobStatusSucceeded || job.Status == queue.JobStatusFailed || job.Status == queue.JobStatusCanceled {
		c.Close()
		return
	}

	// Subscribe to events
	events := h.queueManager.Subscribe(jobID)
	defer h.queueManager.Unsubscribe(jobID, events)

	// Send events to client
	for event := range events {
		if err := c.WriteJSON(event); err != nil {
			return
		}

		// Close connection when job completes
		if event.Status == queue.JobStatusSucceeded || event.Status == queue.JobStatusFailed || event.Status == queue.JobStatusCanceled {
			time.Sleep(100 * time.Millisecond)
			return
		}
	}
}
