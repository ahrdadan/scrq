package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// IdempotencyStore stores idempotency keys to prevent duplicate requests
type IdempotencyStore struct {
	keys map[string]*IdempotencyEntry
	mu   sync.RWMutex
	ttl  time.Duration
}

// IdempotencyEntry represents a stored idempotency key
type IdempotencyEntry struct {
	Key       string      `json:"key"`
	JobID     string      `json:"job_id"`
	Response  interface{} `json:"response"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// NewIdempotencyStore creates a new idempotency store
func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	store := &IdempotencyStore{
		keys: make(map[string]*IdempotencyEntry),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

// Check checks if an idempotency key exists and returns the cached response
func (s *IdempotencyStore) Check(key string) (*IdempotencyEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.keys[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry, true
}

// Store stores an idempotency key with its response
func (s *IdempotencyStore) Store(key, jobID string, response interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.keys[key] = &IdempotencyEntry{
		Key:       key,
		JobID:     jobID,
		Response:  response,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}
}

// Delete removes an idempotency key
func (s *IdempotencyStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, key)
}

// cleanup periodically removes expired entries
func (s *IdempotencyStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, entry := range s.keys {
			if now.After(entry.ExpiresAt) {
				delete(s.keys, key)
			}
		}
		s.mu.Unlock()
	}
}

// GenerateAPIKey generates a random API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "scrq_" + hex.EncodeToString(bytes), nil
}

// HashAPIKey hashes an API key for storage
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// GenerateWebhookSignature generates HMAC signature for webhook payloads
func GenerateWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature verifies a webhook signature
func VerifyWebhookSignature(payload []byte, signature, secret string) bool {
	expected := GenerateWebhookSignature(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
