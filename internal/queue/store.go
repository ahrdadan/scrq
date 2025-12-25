package queue

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Store is an in-memory job store with TTL support
type Store struct {
	jobs           map[string]*Job
	idempotencyMap map[string]string // idempotency_key -> job_id
	mu             sync.RWMutex
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
}

// NewStore creates a new job store
func NewStore() *Store {
	s := &Store{
		jobs:           make(map[string]*Job),
		idempotencyMap: make(map[string]string),
		stopCleanup:    make(chan struct{}),
	}

	// Start TTL cleanup goroutine
	s.startCleanup()

	return s
}

// startCleanup starts the background TTL cleanup
func (s *Store) startCleanup() {
	s.cleanupTicker = time.NewTicker(1 * time.Hour)

	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				s.cleanupExpired()
			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanupExpired removes expired jobs
func (s *Store) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	deleted := 0

	for jobID, job := range s.jobs {
		if job.IsExpired() {
			// Remove from idempotency map if exists
			if job.IdempotencyKey != "" {
				delete(s.idempotencyMap, job.IdempotencyKey)
			}
			delete(s.jobs, jobID)
			deleted++
		}
	}

	if deleted > 0 {
		log.Printf("Cleaned up %d expired jobs (now: %d)", deleted, now)
	}
}

// Stop stops the cleanup goroutine
func (s *Store) Stop() {
	close(s.stopCleanup)
}

// Save saves a job to the store
func (s *Store) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[job.ID] = job

	// Save idempotency mapping if key provided
	if job.IdempotencyKey != "" {
		s.idempotencyMap[job.IdempotencyKey] = job.ID
	}

	return nil
}

// GetByIdempotencyKey retrieves a job by idempotency key
func (s *Store) GetByIdempotencyKey(key string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobID, exists := s.idempotencyMap[key]
	if !exists {
		return nil, false
	}

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, false
	}

	// Check if expired
	if job.IsExpired() {
		return nil, false
	}

	return job, true
}

// Get retrieves a job by ID
func (s *Store) Get(jobID string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Check if expired
	if job.IsExpired() {
		return nil, fmt.Errorf("job expired: %s", jobID)
	}

	return job, nil
}

// Update updates a job in the store
func (s *Store) Update(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[job.ID]; !ok {
		return fmt.Errorf("job not found: %s", job.ID)
	}
	s.jobs[job.ID] = job
	return nil
}

// Delete removes a job from the store
func (s *Store) Delete(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, jobID)
	return nil
}

// List returns all jobs
func (s *Store) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// ToJSON serializes a job to JSON
func (j *Job) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}

// FromJSON deserializes a job from JSON
func FromJSON(data []byte) (*Job, error) {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}
