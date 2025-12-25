package queue

import (
	"sync"
)

// Event represents a job event
type Event struct {
	JobID    string    `json:"job_id"`
	Status   JobStatus `json:"status"`
	Progress int       `json:"progress,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// EventHub manages event subscriptions
type EventHub struct {
	subscribers map[string][]chan Event
	mu          sync.RWMutex
}

// NewEventHub creates a new event hub
func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[string][]chan Event),
	}
}

// Subscribe creates a subscription for job events
func (h *EventHub) Subscribe(jobID string) <-chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, 10)
	h.subscribers[jobID] = append(h.subscribers[jobID], ch)
	return ch
}

// Unsubscribe removes a subscription
func (h *EventHub) Unsubscribe(jobID string, ch <-chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subs := h.subscribers[jobID]
	for i, sub := range subs {
		if sub == ch {
			h.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}

	if len(h.subscribers[jobID]) == 0 {
		delete(h.subscribers, jobID)
	}
}

// Emit sends an event to all subscribers of a job
func (h *EventHub) Emit(jobID string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.subscribers[jobID] {
		select {
		case ch <- event:
		default:
			// Skip if channel is full
		}
	}
}

// Close closes all subscriptions
func (h *EventHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for jobID, subs := range h.subscribers {
		for _, ch := range subs {
			close(ch)
		}
		delete(h.subscribers, jobID)
	}
}
