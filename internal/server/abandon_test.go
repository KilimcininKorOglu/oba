// Package server provides the LDAP server implementation.
package server

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAbandonHandler(t *testing.T) {
	handler := NewAbandonHandler()
	if handler == nil {
		t.Fatal("NewAbandonHandler returned nil")
	}
	if handler.pendingOps == nil {
		t.Error("pendingOps map should not be nil")
	}
	if handler.PendingCount() != 0 {
		t.Errorf("Expected 0 pending operations, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerRegister(t *testing.T) {
	handler := NewAbandonHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := handler.Register(1, cancel)

	if op == nil {
		t.Fatal("Register returned nil")
	}
	if op.MessageID != 1 {
		t.Errorf("Expected MessageID 1, got %d", op.MessageID)
	}
	if op.Cancel == nil {
		t.Error("Cancel function should not be nil")
	}
	if op.Done == nil {
		t.Error("Done channel should not be nil")
	}
	if !handler.IsPending(1) {
		t.Error("Operation should be pending after registration")
	}
	if handler.PendingCount() != 1 {
		t.Errorf("Expected 1 pending operation, got %d", handler.PendingCount())
	}

	// Verify context is not cancelled yet
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled yet")
	default:
		// Expected
	}
}

func TestAbandonHandlerUnregister(t *testing.T) {
	handler := NewAbandonHandler()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler.Register(1, cancel)
	handler.Register(2, cancel)

	if handler.PendingCount() != 2 {
		t.Errorf("Expected 2 pending operations, got %d", handler.PendingCount())
	}

	handler.Unregister(1)

	if handler.IsPending(1) {
		t.Error("Operation 1 should not be pending after unregister")
	}
	if !handler.IsPending(2) {
		t.Error("Operation 2 should still be pending")
	}
	if handler.PendingCount() != 1 {
		t.Errorf("Expected 1 pending operation, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerUnregisterNonExistent(t *testing.T) {
	handler := NewAbandonHandler()

	// Should not panic when unregistering non-existent operation
	handler.Unregister(999)

	if handler.PendingCount() != 0 {
		t.Errorf("Expected 0 pending operations, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerHandle(t *testing.T) {
	handler := NewAbandonHandler()
	ctx, cancel := context.WithCancel(context.Background())

	handler.Register(1, cancel)

	// Handle abandon request
	handler.Handle(nil, 1)

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Expected - context should be cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have been cancelled")
	}
}

func TestAbandonHandlerHandleNonExistent(t *testing.T) {
	handler := NewAbandonHandler()

	// Should not panic when handling abandon for non-existent operation
	handler.Handle(nil, 999)
}

func TestAbandonHandlerIsPending(t *testing.T) {
	handler := NewAbandonHandler()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	if handler.IsPending(1) {
		t.Error("Operation should not be pending before registration")
	}

	handler.Register(1, cancel)

	if !handler.IsPending(1) {
		t.Error("Operation should be pending after registration")
	}

	handler.Unregister(1)

	if handler.IsPending(1) {
		t.Error("Operation should not be pending after unregister")
	}
}

func TestAbandonHandlerGetOperation(t *testing.T) {
	handler := NewAbandonHandler()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Non-existent operation
	if handler.GetOperation(1) != nil {
		t.Error("GetOperation should return nil for non-existent operation")
	}

	op := handler.Register(1, cancel)
	retrieved := handler.GetOperation(1)

	if retrieved == nil {
		t.Fatal("GetOperation returned nil for existing operation")
	}
	if retrieved != op {
		t.Error("GetOperation should return the same operation that was registered")
	}
}

func TestAbandonHandlerCancelAll(t *testing.T) {
	handler := NewAbandonHandler()

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())

	handler.Register(1, cancel1)
	handler.Register(2, cancel2)
	handler.Register(3, cancel3)

	if handler.PendingCount() != 3 {
		t.Errorf("Expected 3 pending operations, got %d", handler.PendingCount())
	}

	handler.CancelAll()

	// Verify all contexts were cancelled
	select {
	case <-ctx1.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context 1 should have been cancelled")
	}

	select {
	case <-ctx2.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context 2 should have been cancelled")
	}

	select {
	case <-ctx3.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context 3 should have been cancelled")
	}

	// Verify all operations were removed
	if handler.PendingCount() != 0 {
		t.Errorf("Expected 0 pending operations after CancelAll, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerDoneChannel(t *testing.T) {
	handler := NewAbandonHandler()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := handler.Register(1, cancel)

	// Done channel should not be closed yet
	select {
	case <-op.Done:
		t.Error("Done channel should not be closed before unregister")
	default:
		// Expected
	}

	handler.Unregister(1)

	// Done channel should be closed after unregister
	select {
	case <-op.Done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed after unregister")
	}
}

func TestAbandonHandlerConcurrentAccess(t *testing.T) {
	handler := NewAbandonHandler()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, cancel := context.WithCancel(context.Background())
			handler.Register(id, cancel)
		}(i)
	}
	wg.Wait()

	if handler.PendingCount() != numGoroutines {
		t.Errorf("Expected %d pending operations, got %d", numGoroutines, handler.PendingCount())
	}

	// Concurrent unregistrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handler.Unregister(id)
		}(i)
	}
	wg.Wait()

	if handler.PendingCount() != 0 {
		t.Errorf("Expected 0 pending operations, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerConcurrentHandleAndUnregister(t *testing.T) {
	handler := NewAbandonHandler()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		handler.Register(i, cancel)

		wg.Add(2)
		// Concurrent handle
		go func(id int) {
			defer wg.Done()
			handler.Handle(nil, id)
		}(i)

		// Concurrent unregister
		go func(id int, ctx context.Context) {
			defer wg.Done()
			// Wait a bit for potential cancellation
			time.Sleep(time.Millisecond)
			handler.Unregister(id)
		}(i, ctx)
	}

	wg.Wait()

	if handler.PendingCount() != 0 {
		t.Errorf("Expected 0 pending operations, got %d", handler.PendingCount())
	}
}

func TestAbandonHandlerMultipleUnregister(t *testing.T) {
	handler := NewAbandonHandler()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := handler.Register(1, cancel)

	// First unregister
	handler.Unregister(1)

	// Done channel should be closed
	select {
	case <-op.Done:
		// Expected
	default:
		t.Error("Done channel should be closed after first unregister")
	}

	// Second unregister should not panic
	handler.Unregister(1)
}

func TestAbandonHandlerIntegrationWithSearch(t *testing.T) {
	// Simulate a search operation that can be abandoned
	handler := NewAbandonHandler()

	var searchCompleted atomic.Bool
	var searchCancelled atomic.Bool

	// Simulate a long-running search
	ctx, cancel := context.WithCancel(context.Background())
	handler.Register(1, cancel)

	go func() {
		defer handler.Unregister(1)

		// Simulate search iteration
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				searchCancelled.Store(true)
				return
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
		searchCompleted.Store(true)
	}()

	// Wait a bit then abandon
	time.Sleep(50 * time.Millisecond)
	handler.Handle(nil, 1)

	// Wait for search to finish
	time.Sleep(100 * time.Millisecond)

	if !searchCancelled.Load() {
		t.Error("Search should have been cancelled")
	}
	if searchCompleted.Load() {
		t.Error("Search should not have completed normally")
	}
}

func TestPendingOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := &PendingOperation{
		MessageID: 42,
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}

	if op.MessageID != 42 {
		t.Errorf("Expected MessageID 42, got %d", op.MessageID)
	}

	// Test cancellation
	op.Cancel()

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have been cancelled")
	}
}

func TestAbandonHandlerReregisterSameID(t *testing.T) {
	handler := NewAbandonHandler()

	_, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()

	op1 := handler.Register(1, cancel1)
	op2 := handler.Register(1, cancel2) // Re-register with same ID

	// Should have replaced the first operation
	if handler.PendingCount() != 1 {
		t.Errorf("Expected 1 pending operation, got %d", handler.PendingCount())
	}

	retrieved := handler.GetOperation(1)
	if retrieved != op2 {
		t.Error("GetOperation should return the second registered operation")
	}
	if retrieved == op1 {
		t.Error("First operation should have been replaced")
	}

	// Handle should cancel the second context
	handler.Handle(nil, 1)

	select {
	case <-ctx2.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Second context should have been cancelled")
	}
}
