package stream

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Buffer size constants.
const (
	DefaultBufferSize = 256
	ReplayBufferSize  = 4096
)

// Errors.
var (
	ErrTokenTooOld   = errors.New("stream: resume token too old")
	ErrBrokerClosed  = errors.New("stream: broker is closed")
	ErrSubscriberNil = errors.New("stream: subscriber is nil")
)

// Broker manages change event subscriptions and publishing.
type Broker struct {
	subscribers     sync.Map // map[SubscriberID]*Subscriber
	nextID          atomic.Uint64
	subscriberCount atomic.Int64
	replayBuffer    *RingBuffer
	nextToken       atomic.Uint64
	closed          atomic.Bool
}

// NewBroker creates a new change stream broker.
func NewBroker() *Broker {
	return &Broker{
		replayBuffer: NewRingBuffer(ReplayBufferSize),
	}
}

// Subscribe creates a new subscription with the given filter.
// Returns a Subscriber that receives matching events on its Channel.
func (b *Broker) Subscribe(filter WatchFilter) *Subscriber {
	if b.closed.Load() {
		return nil
	}

	id := SubscriberID(b.nextID.Add(1))
	sub := NewSubscriber(id, filter, DefaultBufferSize)
	b.subscribers.Store(id, sub)
	b.subscriberCount.Add(1)
	return sub
}

// SubscribeWithResume creates a subscription and replays events from the given token.
// Returns ErrTokenTooOld if the token is older than the oldest event in the replay buffer.
func (b *Broker) SubscribeWithResume(filter WatchFilter, resumeToken uint64) (*Subscriber, error) {
	if b.closed.Load() {
		return nil, ErrBrokerClosed
	}

	// Get events since token first to check if token is valid
	events := b.replayBuffer.EventsSince(resumeToken)
	if events == nil {
		return nil, ErrTokenTooOld
	}

	// Create subscriber
	sub := b.Subscribe(filter)
	if sub == nil {
		return nil, ErrBrokerClosed
	}

	// Replay matching events
	for _, event := range events {
		if filter.Matches(&event) {
			sub.Send(event)
		}
	}

	return sub, nil
}

// Unsubscribe removes a subscription by ID.
func (b *Broker) Unsubscribe(id SubscriberID) {
	if val, ok := b.subscribers.LoadAndDelete(id); ok {
		sub := val.(*Subscriber)
		sub.Close()
		b.subscriberCount.Add(-1)
	}
}

// Publish sends an event to all matching subscribers.
// The event's Token and Timestamp are set automatically.
func (b *Broker) Publish(event ChangeEvent) {
	if b.closed.Load() {
		return
	}

	// Fast path: no subscribers
	if b.subscriberCount.Load() == 0 {
		// Still add to replay buffer for resume support
		event.Token = b.nextToken.Add(1)
		event.Timestamp = time.Now()
		b.replayBuffer.Push(event)
		return
	}

	// Assign token and timestamp
	event.Token = b.nextToken.Add(1)
	event.Timestamp = time.Now()

	// Add to replay buffer
	b.replayBuffer.Push(event)

	// Broadcast to matching subscribers
	b.subscribers.Range(func(key, value interface{}) bool {
		sub := value.(*Subscriber)
		if sub.Filter.Matches(&event) {
			sub.Send(event)
		}
		return true
	})
}

// HasSubscribers returns true if there are active subscribers.
func (b *Broker) HasSubscribers() bool {
	return b.subscriberCount.Load() > 0
}

// SubscriberCount returns the number of active subscribers.
func (b *Broker) SubscriberCount() int64 {
	return b.subscriberCount.Load()
}

// CurrentToken returns the current (last assigned) token.
func (b *Broker) CurrentToken() uint64 {
	return b.nextToken.Load()
}

// Stats returns broker statistics.
func (b *Broker) Stats() BrokerStats {
	return BrokerStats{
		SubscriberCount: b.subscriberCount.Load(),
		CurrentToken:    b.nextToken.Load(),
		ReplayBufferLen: b.replayBuffer.Len(),
		MinReplayToken:  b.replayBuffer.MinToken(),
	}
}

// Close closes the broker and all subscribers.
func (b *Broker) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}

	b.subscribers.Range(func(key, value interface{}) bool {
		sub := value.(*Subscriber)
		sub.Close()
		b.subscribers.Delete(key)
		return true
	})
	b.subscriberCount.Store(0)
}

// IsClosed returns true if the broker has been closed.
func (b *Broker) IsClosed() bool {
	return b.closed.Load()
}

// BrokerStats contains broker statistics.
type BrokerStats struct {
	SubscriberCount int64
	CurrentToken    uint64
	ReplayBufferLen int
	MinReplayToken  uint64
}
