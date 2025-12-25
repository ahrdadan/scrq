package queue

import (
	"time"

	"github.com/google/uuid"
)

// Default values for job configuration
const (
	DefaultJobTimeout = 30 * time.Second
	DefaultMaxRetries = 3
	DefaultResultTTL  = 7 * 24 * time.Hour // 7 days
	DefaultRetryDelay = 5 * time.Second
	MaxRetryDelay     = 5 * time.Minute
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"
	JobStatusRetrying  JobStatus = "retrying"
)

// JobType represents the type of job
type JobType string

const (
	JobTypeScrape JobType = "scrape"
)

// NotifyConfig holds notification settings for a job
type NotifyConfig struct {
	WebhookURL    string `json:"webhook_url,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"` // For HMAC signature
	WebSocket     bool   `json:"websocket,omitempty"`
}

// RetryConfig holds retry settings for a job
type RetryConfig struct {
	MaxRetries    int     `json:"max_retries"`    // Maximum retry attempts (default: 3)
	RetryDelay    int     `json:"retry_delay"`    // Initial delay between retries in seconds
	BackoffFactor float64 `json:"backoff_factor"` // Exponential backoff multiplier (default: 2.0)
}

// CookieParam represents cookie parameters for requests
type CookieParam struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	URL      string `json:"url,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// ProgressInfo holds detailed progress information
type ProgressInfo struct {
	Current int    `json:"current"` // Current item (e.g., page 5)
	Total   int    `json:"total"`   // Total items (e.g., 20 pages)
	Percent int    `json:"percent"` // Percentage complete (0-100)
	Message string `json:"message"` // Human-readable message

	Stage       string `json:"stage,omitempty"`
	CurrentPage int    `json:"current_page,omitempty"`
	TotalPages  int    `json:"total_pages,omitempty"`
	CurrentItem int    `json:"current_item,omitempty"`
	TotalItems  int    `json:"total_items,omitempty"`
}

// JobRequest represents a job creation request
type JobRequest struct {
	Type           JobType           `json:"type"`
	URL            string            `json:"url"`
	URLs           []string          `json:"urls,omitempty"` // For batch operations
	Engine         string            `json:"engine"`         // lightpanda or chrome
	Timeout        int               `json:"timeout"`        // seconds (default: 30)
	WaitForLoad    bool              `json:"wait_for_load"`
	Script         string            `json:"script,omitempty"`
	UserAgent      string            `json:"user_agent,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Cookies        []CookieParam     `json:"cookies,omitempty"`
	Proxy          string            `json:"proxy,omitempty"` // only for chrome engine
	Notify         *NotifyConfig     `json:"notify,omitempty"`
	Retry          *RetryConfig      `json:"retry,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"` // Client-provided idempotency key
	Priority       int               `json:"priority,omitempty"`        // Job priority (higher = more urgent)
	ResultTTL      int               `json:"result_ttl,omitempty"`      // Result TTL in seconds (default: 7 days)
}

// Job represents a queued job
type Job struct {
	ID             string        `json:"job_id"`
	Type           JobType       `json:"type"`
	Status         JobStatus     `json:"status"`
	Progress       int           `json:"progress"`
	ProgressInfo   *ProgressInfo `json:"progress_info,omitempty"`
	Message        string        `json:"message,omitempty"`
	Request        JobRequest    `json:"request"`
	Result         interface{}   `json:"result,omitempty"`
	Error          string        `json:"error,omitempty"`
	CreatedAt      int64         `json:"created_at"`
	UpdatedAt      int64         `json:"updated_at"`
	StartedAt      int64         `json:"started_at,omitempty"`
	CompletedAt    int64         `json:"completed_at,omitempty"`
	ExpiresAt      int64         `json:"expires_at,omitempty"` // When result will be deleted
	Notify         *NotifyConfig `json:"notify,omitempty"`
	RetryCount     int           `json:"retry_count"`
	MaxRetries     int           `json:"max_retries"`
	NextRetryAt    int64         `json:"next_retry_at,omitempty"`
	LastError      string        `json:"last_error,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
	Priority       int           `json:"priority"`
	UserID         string        `json:"user_id,omitempty"` // For rate limiting
	Timeout        int           `json:"timeout"`           // Job timeout in seconds
}

// NewJob creates a new job from a request
func NewJob(req JobRequest) *Job {
	now := time.Now().Unix()

	// Set default timeout
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = int(DefaultJobTimeout.Seconds())
	}

	// Set default max retries
	maxRetries := DefaultMaxRetries
	if req.Retry != nil && req.Retry.MaxRetries > 0 {
		maxRetries = req.Retry.MaxRetries
	}

	// Calculate expiry time
	resultTTL := DefaultResultTTL
	if req.ResultTTL > 0 {
		resultTTL = time.Duration(req.ResultTTL) * time.Second
	}
	expiresAt := time.Now().Add(resultTTL).Unix()

	return &Job{
		ID:             generateJobID(),
		Type:           req.Type,
		Status:         JobStatusQueued,
		Progress:       0,
		Request:        req,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      expiresAt,
		Notify:         req.Notify,
		MaxRetries:     maxRetries,
		RetryCount:     0,
		IdempotencyKey: req.IdempotencyKey,
		Priority:       req.Priority,
		Timeout:        timeout,
	}
}

// SetStatus updates the job status
func (j *Job) SetStatus(status JobStatus) {
	j.Status = status
	j.UpdatedAt = time.Now().Unix()

	if status == JobStatusRunning && j.StartedAt == 0 {
		j.StartedAt = time.Now().Unix()
	}

	if status == JobStatusSucceeded || status == JobStatusFailed || status == JobStatusCanceled {
		j.CompletedAt = time.Now().Unix()
	}
}

// SetProgress updates the job progress
func (j *Job) SetProgress(progress int, message string) {
	j.Progress = progress
	j.Message = message
	j.UpdatedAt = time.Now().Unix()
}

// SetProgressInfo updates detailed progress information
func (j *Job) SetProgressInfo(current, total int, message string) {
	percent := 0
	if total > 0 {
		percent = (current * 100) / total
	}

	j.Progress = percent
	j.Message = message
	j.ProgressInfo = &ProgressInfo{
		Current: current,
		Total:   total,
		Percent: percent,
		Message: message,
	}
	j.UpdatedAt = time.Now().Unix()
}

// SetResult sets the job result
func (j *Job) SetResult(result interface{}) {
	j.Result = result
	j.Status = JobStatusSucceeded
	j.Progress = 100
	j.CompletedAt = time.Now().Unix()
	j.UpdatedAt = time.Now().Unix()
}

// SetError sets the job error
func (j *Job) SetError(err string) {
	j.Error = err
	j.LastError = err
	j.Status = JobStatusFailed
	j.CompletedAt = time.Now().Unix()
	j.UpdatedAt = time.Now().Unix()
}

// CanRetry returns true if the job can be retried
func (j *Job) CanRetry() bool {
	return j.RetryCount < j.MaxRetries
}

// PrepareRetry prepares the job for retry
func (j *Job) PrepareRetry() {
	j.RetryCount++
	j.Status = JobStatusRetrying

	// Calculate next retry time with exponential backoff
	backoffFactor := 2.0
	if j.Request.Retry != nil && j.Request.Retry.BackoffFactor > 0 {
		backoffFactor = j.Request.Retry.BackoffFactor
	}

	baseDelay := DefaultRetryDelay
	if j.Request.Retry != nil && j.Request.Retry.RetryDelay > 0 {
		baseDelay = time.Duration(j.Request.Retry.RetryDelay) * time.Second
	}

	// Calculate delay: baseDelay * (backoffFactor ^ retryCount)
	delay := baseDelay
	for i := 1; i < j.RetryCount; i++ {
		delay = time.Duration(float64(delay) * backoffFactor)
	}

	// Cap at max delay
	if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}

	j.NextRetryAt = time.Now().Add(delay).Unix()
	j.UpdatedAt = time.Now().Unix()
}

// IsExpired checks if the job result has expired
func (j *Job) IsExpired() bool {
	if j.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > j.ExpiresAt
}

// GetTimeoutDuration returns the job timeout as a time.Duration
func (j *Job) GetTimeoutDuration() time.Duration {
	if j.Timeout <= 0 {
		return DefaultJobTimeout
	}
	return time.Duration(j.Timeout) * time.Second
}

// JobStatusResponse represents a job status response
type JobStatusResponse struct {
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message,omitempty"`
	CreatedAt int64     `json:"created_at"`
	UpdatedAt int64     `json:"updated_at"`
}

// JobResultResponse represents a job result response
type JobResultResponse struct {
	JobID  string      `json:"job_id"`
	Status JobStatus   `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// JobCreatedResponse represents the response when a job is created
type JobCreatedResponse struct {
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	StatusURL string    `json:"status_url"`
	ResultURL string    `json:"result_url"`
	Events    struct {
		SSEURL string `json:"sse_url"`
		WSURL  string `json:"ws_url"`
	} `json:"events"`
}

func generateJobID() string {
	return "job_" + uuid.New().String()[:8]
}
