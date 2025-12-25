package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	// StreamName is the name of the JetStream stream
	StreamName = "SCRQ_JOBS"
	// SubjectName is the subject for job messages
	SubjectName = "scrq.jobs"
	// ConsumerName is the name of the durable consumer
	ConsumerName = "scrq-worker"
)

// Manager manages the job queue
type Manager struct {
	js        jetstream.JetStream
	store     *Store
	events    *EventHub
	stream    jetstream.Stream
	consumer  jetstream.Consumer
	mu        sync.Mutex
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new queue manager
func NewManager(js jetstream.JetStream) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		js:     js,
		store:  NewStore(),
		events: NewEventHub(),
		ctx:    ctx,
		cancel: cancel,
	}

	if err := m.setupStream(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup stream: %w", err)
	}

	return m, nil
}

// setupStream creates or updates the JetStream stream
func (m *Manager) setupStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create or update stream
	stream, err := m.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        StreamName,
		Description: "Scrq job queue",
		Subjects:    []string{SubjectName},
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      24 * time.Hour,
		Storage:     jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	m.stream = stream

	// Create or update consumer
	consumer, err := m.js.CreateOrUpdateConsumer(ctx, StreamName, jetstream.ConsumerConfig{
		Name:          ConsumerName,
		Durable:       ConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		MaxDeliver:    3,
		AckWait:       5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	m.consumer = consumer

	return nil
}

// Start starts processing jobs from the queue
func (m *Manager) Start(processor JobProcessor) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.isRunning = true
	m.mu.Unlock()

	log.Println("Starting job queue worker...")

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			default:
				msgs, err := m.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
				if err != nil {
					continue
				}

				for msg := range msgs.Messages() {
					m.processMessage(msg, processor)
				}
			}
		}
	}()

	return nil
}

// Stop stops the queue manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return
	}

	m.cancel()
	m.isRunning = false
	log.Println("Job queue worker stopped")
}

// Enqueue adds a job to the queue
func (m *Manager) Enqueue(job *Job) error {
	// Save job to store
	if err := m.store.Save(job); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}

	// Publish to JetStream
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := m.js.Publish(ctx, SubjectName, data); err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	// Emit event
	m.events.Emit(job.ID, Event{
		JobID:   job.ID,
		Status:  job.Status,
		Message: "Job queued",
	})

	return nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(jobID string) (*Job, error) {
	return m.store.Get(jobID)
}

// UpdateJob updates a job and emits an event
func (m *Manager) UpdateJob(job *Job) error {
	if err := m.store.Update(job); err != nil {
		return err
	}

	m.events.Emit(job.ID, Event{
		JobID:    job.ID,
		Status:   job.Status,
		Progress: job.Progress,
		Message:  job.Message,
	})

	return nil
}

// CancelJob cancels a job
func (m *Manager) CancelJob(jobID string) (*Job, error) {
	job, err := m.store.Get(jobID)
	if err != nil {
		return nil, err
	}

	if job.Status != JobStatusQueued && job.Status != JobStatusRunning {
		return nil, fmt.Errorf("cannot cancel job with status: %s", job.Status)
	}

	job.SetStatus(JobStatusCanceled)
	if err := m.store.Update(job); err != nil {
		return nil, err
	}

	m.events.Emit(job.ID, Event{
		JobID:   job.ID,
		Status:  job.Status,
		Message: "Job canceled",
	})

	return job, nil
}

// Subscribe subscribes to job events
func (m *Manager) Subscribe(jobID string) <-chan Event {
	return m.events.Subscribe(jobID)
}

// Unsubscribe unsubscribes from job events
func (m *Manager) Unsubscribe(jobID string, ch <-chan Event) {
	m.events.Unsubscribe(jobID, ch)
}

// GetEventHub returns the event hub
func (m *Manager) GetEventHub() *EventHub {
	return m.events
}

// GetStore returns the job store
func (m *Manager) GetStore() *Store {
	return m.store
}

// EnqueueWithIdempotency enqueues a job with idempotency check
func (m *Manager) EnqueueWithIdempotency(job *Job) (*Job, bool, error) {
	// Check for existing job with same idempotency key
	if job.IdempotencyKey != "" {
		existingJob, exists := m.store.GetByIdempotencyKey(job.IdempotencyKey)
		if exists {
			return existingJob, true, nil // Return existing job, was duplicate
		}
	}

	if err := m.Enqueue(job); err != nil {
		return nil, false, err
	}

	return job, false, nil
}

func (m *Manager) processMessage(msg jetstream.Msg, processor JobProcessor) {
	var job Job
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		log.Printf("Failed to unmarshal job: %v", err)
		msg.Nak()
		return
	}

	// Check if job was canceled
	storedJob, err := m.store.Get(job.ID)
	if err != nil {
		log.Printf("Failed to get job from store: %v", err)
		msg.Nak()
		return
	}

	if storedJob.Status == JobStatusCanceled {
		msg.Ack()
		return
	}

	// Check if we need to wait for retry delay
	if storedJob.Status == JobStatusRetrying && storedJob.NextRetryAt > 0 {
		waitUntil := time.Unix(storedJob.NextRetryAt, 0)
		if time.Now().Before(waitUntil) {
			// Re-queue with delay
			msg.NakWithDelay(time.Until(waitUntil))
			return
		}
	}

	// Update status to running
	storedJob.SetStatus(JobStatusRunning)
	storedJob.SetProgress(0, "Processing started")
	m.UpdateJob(storedJob)

	// Create context with timeout
	timeout := storedJob.GetTimeoutDuration()
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	// Process the job with progress callback that supports page X/Y
	result, err := processor.Process(ctx, storedJob, func(progress int, message string) {
		storedJob.SetProgress(progress, message)
		m.UpdateJob(storedJob)
	})

	if err != nil {
		// Check if we can retry
		if storedJob.CanRetry() {
			storedJob.LastError = err.Error()
			storedJob.PrepareRetry()
			m.UpdateJob(storedJob)

			// Emit retry event
			m.events.Emit(storedJob.ID, Event{
				JobID:    storedJob.ID,
				Status:   storedJob.Status,
				Progress: storedJob.Progress,
				Message:  fmt.Sprintf("Retrying (%d/%d): %s", storedJob.RetryCount, storedJob.MaxRetries, err.Error()),
			})

			// Re-enqueue for retry
			data, _ := storedJob.ToJSON()
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer retryCancel()

			if _, pubErr := m.js.Publish(retryCtx, SubjectName, data); pubErr != nil {
				log.Printf("Failed to re-enqueue job for retry: %v", pubErr)
			}

			msg.Ack()
			return
		}

		storedJob.SetError(err.Error())
		m.UpdateJob(storedJob)
		msg.Ack()
		return
	}

	storedJob.SetResult(result)
	m.UpdateJob(storedJob)
	msg.Ack()
}

// JobProcessor defines the interface for processing jobs
type JobProcessor interface {
	Process(ctx context.Context, job *Job, progress func(int, string)) (interface{}, error)
}

// ProgressCallback is a function for reporting progress with page info
type ProgressCallback func(current, total int, message string)
