package stream

import "sync"

// RingBuffer is a fixed-size circular buffer for storing recent events.
// It supports token-based resume by keeping track of the minimum token.
type RingBuffer struct {
	events   []ChangeEvent
	head     int
	tail     int
	size     int
	capacity int
	minToken uint64
	mu       sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = ReplayBufferSize
	}
	return &RingBuffer{
		events:   make([]ChangeEvent, capacity),
		capacity: capacity,
	}
}

// Push adds an event to the buffer.
// If the buffer is full, the oldest event is overwritten.
func (rb *RingBuffer) Push(event ChangeEvent) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.events[rb.tail] = event
	rb.tail = (rb.tail + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
		if rb.size == 1 {
			rb.minToken = event.Token
		}
	} else {
		rb.head = (rb.head + 1) % rb.capacity
		rb.minToken = rb.events[rb.head].Token
	}
}

// EventsSince returns all events with tokens greater than the given token.
// Returns nil if the token is too old (older than the oldest event in buffer).
// Returns empty slice if no events match.
func (rb *RingBuffer) EventsSince(token uint64) []ChangeEvent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.size == 0 {
		return []ChangeEvent{}
	}

	// Token 0 means get all events
	if token > 0 && token < rb.minToken {
		return nil // Token too old
	}

	var result []ChangeEvent
	for i := 0; i < rb.size; i++ {
		idx := (rb.head + i) % rb.capacity
		if rb.events[idx].Token > token {
			result = append(result, rb.events[idx])
		}
	}
	return result
}

// Len returns the number of events in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// MinToken returns the minimum token in the buffer.
// Returns 0 if the buffer is empty.
func (rb *RingBuffer) MinToken() uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.size == 0 {
		return 0
	}
	return rb.minToken
}

// MaxToken returns the maximum token in the buffer.
// Returns 0 if the buffer is empty.
func (rb *RingBuffer) MaxToken() uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.size == 0 {
		return 0
	}
	lastIdx := (rb.tail - 1 + rb.capacity) % rb.capacity
	return rb.events[lastIdx].Token
}

// Clear removes all events from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.size = 0
	rb.minToken = 0
}
