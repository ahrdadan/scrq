package security

import (
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	windows  map[string]*Window
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	burstMax int
}

// Window represents a rate limit window for a specific key
type Window struct {
	Requests  []time.Time
	LastReset time.Time
}

// RateLimitConfig holds rate limiter configuration
type RateLimitConfig struct {
	// RequestsPerWindow is the maximum number of requests allowed per window
	RequestsPerWindow int
	// WindowDuration is the duration of the rate limit window
	WindowDuration time.Duration
	// BurstMax is the maximum burst size allowed
	BurstMax int
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerWindow: 100,         // 100 requests
		WindowDuration:    time.Minute, // per minute
		BurstMax:          20,          // burst of 20
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		windows:  make(map[string]*Window),
		limit:    config.RequestsPerWindow,
		window:   config.WindowDuration,
		burstMax: config.BurstMax,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given key (e.g., user ID, IP)
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get or create window
	w, exists := rl.windows[key]
	if !exists {
		w = &Window{
			Requests:  make([]time.Time, 0, rl.limit),
			LastReset: now,
		}
		rl.windows[key] = w
	}

	// Remove old requests outside the window
	valid := make([]time.Time, 0, len(w.Requests))
	for _, t := range w.Requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	w.Requests = valid

	// Check if limit exceeded
	if len(w.Requests) >= rl.limit {
		return false
	}

	// Check burst limit (requests in last second)
	burstCutoff := now.Add(-time.Second)
	burstCount := 0
	for _, t := range w.Requests {
		if t.After(burstCutoff) {
			burstCount++
		}
	}
	if burstCount >= rl.burstMax {
		return false
	}

	// Add request
	w.Requests = append(w.Requests, now)
	return true
}

// GetRemainingRequests returns the number of remaining requests for a key
func (rl *RateLimiter) GetRemainingRequests(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	w, exists := rl.windows[key]
	if !exists {
		return rl.limit
	}

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Count valid requests
	count := 0
	for _, t := range w.Requests {
		if t.After(cutoff) {
			count++
		}
	}

	remaining := rl.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetResetTime returns the time when the rate limit will reset for a key
func (rl *RateLimiter) GetResetTime(key string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	w, exists := rl.windows[key]
	if !exists || len(w.Requests) == 0 {
		return time.Now()
	}

	// Return when oldest request will expire
	return w.Requests[0].Add(rl.window)
}

// Reset resets the rate limit for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, key)
}

// cleanup periodically removes stale windows
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window * 2)

		for key, w := range rl.windows {
			if w.LastReset.Before(cutoff) && len(w.Requests) == 0 {
				delete(rl.windows, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitInfo contains rate limit information for response headers
type RateLimitInfo struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
}

// GetInfo returns rate limit info for a key
func (rl *RateLimiter) GetInfo(key string) RateLimitInfo {
	return RateLimitInfo{
		Limit:     rl.limit,
		Remaining: rl.GetRemainingRequests(key),
		ResetAt:   rl.GetResetTime(key),
	}
}
