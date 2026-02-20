package stream

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
)

func TestOperationTypeString(t *testing.T) {
	tests := []struct {
		op   OperationType
		want string
	}{
		{OpInsert, "insert"},
		{OpUpdate, "update"},
		{OpDelete, "delete"},
		{OpModifyDN, "modifyDN"},
		{OperationType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("OperationType(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestWatchFilterMatches(t *testing.T) {
	tests := []struct {
		name    string
		filter  WatchFilter
		event   *ChangeEvent
		matches bool
	}{
		{
			name:    "empty filter matches all",
			filter:  WatchFilter{},
			event:   &ChangeEvent{DN: "cn=test,dc=example,dc=com"},
			matches: true,
		},
		{
			name:    "base scope exact match",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeBase},
			event:   &ChangeEvent{DN: "dc=example,dc=com"},
			matches: true,
		},
		{
			name:    "base scope no match",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeBase},
			event:   &ChangeEvent{DN: "cn=test,dc=example,dc=com"},
			matches: false,
		},
		{
			name:    "one level direct child",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeOneLevel},
			event:   &ChangeEvent{DN: "cn=test,dc=example,dc=com"},
			matches: true,
		},
		{
			name:    "one level nested child no match",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeOneLevel},
			event:   &ChangeEvent{DN: "cn=test,ou=users,dc=example,dc=com"},
			matches: false,
		},
		{
			name:    "subtree matches base",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeSubtree},
			event:   &ChangeEvent{DN: "dc=example,dc=com"},
			matches: true,
		},
		{
			name:    "subtree matches nested",
			filter:  WatchFilter{BaseDN: "dc=example,dc=com", Scope: ScopeSubtree},
			event:   &ChangeEvent{DN: "cn=test,ou=users,dc=example,dc=com"},
			matches: true,
		},
		{
			name:    "operation filter match",
			filter:  WatchFilter{Operations: []OperationType{OpInsert, OpUpdate}},
			event:   &ChangeEvent{DN: "cn=test", Operation: OpInsert},
			matches: true,
		},
		{
			name:    "operation filter no match",
			filter:  WatchFilter{Operations: []OperationType{OpInsert}},
			event:   &ChangeEvent{DN: "cn=test", Operation: OpDelete},
			matches: false,
		},
		{
			name:    "nil event",
			filter:  WatchFilter{},
			event:   nil,
			matches: false,
		},
		{
			name:    "case insensitive DN match",
			filter:  WatchFilter{BaseDN: "DC=Example,DC=COM", Scope: ScopeSubtree},
			event:   &ChangeEvent{DN: "cn=test,dc=example,dc=com"},
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(tt.event); got != tt.matches {
				t.Errorf("Matches() = %v, want %v", got, tt.matches)
			}
		})
	}
}

func TestSubscriberSend(t *testing.T) {
	sub := NewSubscriber(1, WatchFilter{}, 2)
	defer sub.Close()

	// Send should succeed
	event := ChangeEvent{DN: "cn=test", Operation: OpInsert}
	if !sub.Send(event) {
		t.Error("Send() should succeed")
	}

	// Receive the event
	select {
	case received := <-sub.Channel:
		if received.DN != event.DN {
			t.Errorf("received DN = %q, want %q", received.DN, event.DN)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestSubscriberBackpressure(t *testing.T) {
	sub := NewSubscriber(1, WatchFilter{}, 2)
	defer sub.Close()

	// Fill the buffer
	sub.Send(ChangeEvent{DN: "cn=1"})
	sub.Send(ChangeEvent{DN: "cn=2"})

	// This should fail due to backpressure
	if sub.Send(ChangeEvent{DN: "cn=3"}) {
		t.Error("Send() should fail when buffer is full")
	}

	if sub.DroppedCount() != 1 {
		t.Errorf("DroppedCount() = %d, want 1", sub.DroppedCount())
	}
}

func TestSubscriberClose(t *testing.T) {
	sub := NewSubscriber(1, WatchFilter{}, 2)

	sub.Close()

	if !sub.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	// Send should fail after close
	if sub.Send(ChangeEvent{DN: "cn=test"}) {
		t.Error("Send() should fail after Close()")
	}

	// Double close should be safe
	sub.Close()
}

func TestRingBufferPush(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push(ChangeEvent{Token: 1, DN: "cn=1"})
	rb.Push(ChangeEvent{Token: 2, DN: "cn=2"})
	rb.Push(ChangeEvent{Token: 3, DN: "cn=3"})

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	// Push one more, should overwrite oldest
	rb.Push(ChangeEvent{Token: 4, DN: "cn=4"})

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	if rb.MinToken() != 2 {
		t.Errorf("MinToken() = %d, want 2", rb.MinToken())
	}

	if rb.MaxToken() != 4 {
		t.Errorf("MaxToken() = %d, want 4", rb.MaxToken())
	}
}

func TestRingBufferEventsSince(t *testing.T) {
	rb := NewRingBuffer(5)

	for i := uint64(1); i <= 5; i++ {
		rb.Push(ChangeEvent{Token: i, DN: fmt.Sprintf("cn=%d", i)})
	}

	// Get events since token 2
	events := rb.EventsSince(2)
	if len(events) != 3 {
		t.Errorf("EventsSince(2) returned %d events, want 3", len(events))
	}

	// Get all events (token 0)
	events = rb.EventsSince(0)
	if len(events) != 5 {
		t.Errorf("EventsSince(0) returned %d events, want 5", len(events))
	}

	// After overflow, token 1 should be too old
	rb.Push(ChangeEvent{Token: 6, DN: "cn=6"})
	// minToken is now 2, so token 1 is too old
	events = rb.EventsSince(1)
	if events != nil {
		t.Error("EventsSince(1) should return nil when token is too old")
	}
}

func TestBrokerSubscribe(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	sub := broker.Subscribe(WatchFilter{})
	if sub == nil {
		t.Fatal("Subscribe() returned nil")
	}

	if broker.SubscriberCount() != 1 {
		t.Errorf("SubscriberCount() = %d, want 1", broker.SubscriberCount())
	}

	broker.Unsubscribe(sub.ID)

	if broker.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount() = %d, want 0", broker.SubscriberCount())
	}
}

func TestBrokerPublish(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	sub := broker.Subscribe(WatchFilter{})
	defer broker.Unsubscribe(sub.ID)

	entry := storage.NewEntry("cn=test,dc=example,dc=com")
	broker.Publish(ChangeEvent{
		Operation: OpInsert,
		DN:        "cn=test,dc=example,dc=com",
		Entry:     entry,
	})

	select {
	case event := <-sub.Channel:
		if event.Operation != OpInsert {
			t.Errorf("Operation = %v, want %v", event.Operation, OpInsert)
		}
		if event.DN != "cn=test,dc=example,dc=com" {
			t.Errorf("DN = %q, want %q", event.DN, "cn=test,dc=example,dc=com")
		}
		if event.Token == 0 {
			t.Error("Token should be assigned")
		}
		if event.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBrokerPublishWithFilter(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	// Subscribe only to users subtree
	sub := broker.Subscribe(WatchFilter{
		BaseDN: "ou=users,dc=example,dc=com",
		Scope:  ScopeSubtree,
	})
	defer broker.Unsubscribe(sub.ID)

	// Publish event outside filter
	broker.Publish(ChangeEvent{
		Operation: OpInsert,
		DN:        "cn=admin,dc=example,dc=com",
	})

	// Should not receive
	select {
	case <-sub.Channel:
		t.Error("should not receive filtered event")
	case <-time.After(100 * time.Millisecond):
		// OK
	}

	// Publish event inside filter
	broker.Publish(ChangeEvent{
		Operation: OpInsert,
		DN:        "cn=user1,ou=users,dc=example,dc=com",
	})

	// Should receive
	select {
	case event := <-sub.Channel:
		if event.DN != "cn=user1,ou=users,dc=example,dc=com" {
			t.Errorf("DN = %q, want %q", event.DN, "cn=user1,ou=users,dc=example,dc=com")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBrokerSubscribeWithResume(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	// Publish some events
	for i := 0; i < 10; i++ {
		broker.Publish(ChangeEvent{
			Operation: OpInsert,
			DN:        fmt.Sprintf("cn=user%d,dc=example,dc=com", i),
		})
	}

	// Subscribe with resume from token 5
	sub, err := broker.SubscribeWithResume(WatchFilter{}, 5)
	if err != nil {
		t.Fatalf("SubscribeWithResume() error = %v", err)
	}
	defer broker.Unsubscribe(sub.ID)

	// Should receive events 6-10 (5 events)
	count := 0
	timeout := time.After(time.Second)
loop:
	for {
		select {
		case event := <-sub.Channel:
			count++
			if event.Token <= 5 {
				t.Errorf("received event with token %d, should be > 5", event.Token)
			}
			if count >= 5 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	if count != 5 {
		t.Errorf("received %d events, want 5", count)
	}
}

func TestBrokerSubscribeWithResumeTokenTooOld(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	// Fill replay buffer
	for i := 0; i < ReplayBufferSize+100; i++ {
		broker.Publish(ChangeEvent{
			Operation: OpInsert,
			DN:        fmt.Sprintf("cn=user%d", i),
		})
	}

	// Try to resume from token 1 (too old)
	_, err := broker.SubscribeWithResume(WatchFilter{}, 1)
	if err != ErrTokenTooOld {
		t.Errorf("SubscribeWithResume() error = %v, want ErrTokenTooOld", err)
	}
}

func TestBrokerClose(t *testing.T) {
	broker := NewBroker()

	sub := broker.Subscribe(WatchFilter{})

	broker.Close()

	if !broker.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	if !sub.IsClosed() {
		t.Error("subscriber should be closed when broker closes")
	}

	// Subscribe should return nil after close
	if broker.Subscribe(WatchFilter{}) != nil {
		t.Error("Subscribe() should return nil after Close()")
	}
}

func TestBrokerConcurrent(t *testing.T) {
	broker := NewBroker()
	defer broker.Close()

	var wg sync.WaitGroup

	// Start 10 subscribers
	subs := make([]*Subscriber, 10)
	for i := 0; i < 10; i++ {
		subs[i] = broker.Subscribe(WatchFilter{})
	}

	// Publish 100 events concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				broker.Publish(ChangeEvent{
					Operation: OpInsert,
					DN:        fmt.Sprintf("cn=user%d-%d", n, j),
				})
			}
		}(i)
	}

	wg.Wait()

	// Each subscriber should receive 100 events
	for i, sub := range subs {
		count := 0
		timeout := time.After(time.Second)
	loop:
		for {
			select {
			case <-sub.Channel:
				count++
				if count >= 100 {
					break loop
				}
			case <-timeout:
				break loop
			}
		}
		if count != 100 {
			t.Errorf("subscriber %d received %d events, want 100", i, count)
		}
		broker.Unsubscribe(sub.ID)
	}
}

func TestMatchHelpers(t *testing.T) {
	// MatchAll
	filter := MatchAll()
	if filter.BaseDN != "" || filter.Scope != 0 {
		t.Error("MatchAll() should return empty filter")
	}

	// MatchDN
	filter = MatchDN("cn=test,dc=example,dc=com")
	if filter.BaseDN != "cn=test,dc=example,dc=com" || filter.Scope != ScopeBase {
		t.Error("MatchDN() should set BaseDN and ScopeBase")
	}

	// MatchSubtree
	filter = MatchSubtree("dc=example,dc=com")
	if filter.BaseDN != "dc=example,dc=com" || filter.Scope != ScopeSubtree {
		t.Error("MatchSubtree() should set BaseDN and ScopeSubtree")
	}
}
