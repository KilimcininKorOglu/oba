package stream

import (
	"sync/atomic"
	"time"
)

// SubscriberID is a unique identifier for a subscriber.
type SubscriberID uint64

// Subscriber represents a change stream subscription.
type Subscriber struct {
	// ID is the unique identifier for this subscriber.
	ID SubscriberID
	// Filter determines which events this subscriber receives.
	Filter WatchFilter
	// Channel receives matching change events.
	Channel chan ChangeEvent
	// Created is when the subscription was created.
	Created time.Time

	dropped atomic.Uint64
	closed  atomic.Bool
}

// NewSubscriber creates a new subscriber with the given filter and buffer size.
func NewSubscriber(id SubscriberID, filter WatchFilter, bufferSize int) *Subscriber {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	return &Subscriber{
		ID:      id,
		Filter:  filter,
		Channel: make(chan ChangeEvent, bufferSize),
		Created: time.Now(),
	}
}

// Send attempts to send an event to the subscriber.
// Returns true if the event was sent, false if the channel is full (backpressure).
func (s *Subscriber) Send(event ChangeEvent) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case s.Channel <- event:
		return true
	default:
		s.dropped.Add(1)
		return false
	}
}

// Close closes the subscriber's channel.
// Safe to call multiple times.
func (s *Subscriber) Close() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.Channel)
	}
}

// IsClosed returns true if the subscriber has been closed.
func (s *Subscriber) IsClosed() bool {
	return s.closed.Load()
}

// DroppedCount returns the number of events dropped due to backpressure.
func (s *Subscriber) DroppedCount() uint64 {
	return s.dropped.Load()
}

// ResetDropped resets the dropped counter and returns the previous value.
func (s *Subscriber) ResetDropped() uint64 {
	return s.dropped.Swap(0)
}
