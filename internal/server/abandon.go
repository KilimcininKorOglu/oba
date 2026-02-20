// Package server provides the LDAP server implementation.
package server

import (
	"context"
	"sync"
)

// PendingOperation represents an operation that can be abandoned.
// It tracks the message ID, cancellation function, and completion channel.
type PendingOperation struct {
	// MessageID is the LDAP message ID of the operation
	MessageID int
	// Cancel is the function to call to cancel the operation
	Cancel context.CancelFunc
	// Done is closed when the operation completes
	Done chan struct{}
}

// AbandonHandler manages pending operations and handles abandon requests.
// Per RFC 4511 Section 4.11, the Abandon operation allows a client to
// request that the server abandon an outstanding operation.
//
// The Abandon operation does not have a response. Once the server receives
// an Abandon request, it should make a best-effort attempt to abandon the
// specified operation, but there is no guarantee that the operation will
// be abandoned.
type AbandonHandler struct {
	// pendingOps maps message IDs to pending operations
	pendingOps map[int]*PendingOperation
	// mu protects concurrent access to pendingOps
	mu sync.RWMutex
}

// NewAbandonHandler creates a new AbandonHandler.
func NewAbandonHandler() *AbandonHandler {
	return &AbandonHandler{
		pendingOps: make(map[int]*PendingOperation),
	}
}

// Handle processes an abandon request for the specified message ID.
// Per RFC 4511, the Abandon operation does not have a response.
// If the operation exists, it will be cancelled.
func (h *AbandonHandler) Handle(conn *Connection, messageID int) {
	h.mu.RLock()
	op, exists := h.pendingOps[messageID]
	h.mu.RUnlock()

	if exists {
		// Cancel the operation
		op.Cancel()
		// Note: We don't unregister here - the operation itself should
		// unregister when it detects cancellation and cleans up
	}
	// No response is sent for Abandon requests per RFC 4511
}

// Register registers a pending operation with the given message ID.
// The cancel function will be called if an abandon request is received
// for this message ID.
func (h *AbandonHandler) Register(messageID int, cancel context.CancelFunc) *PendingOperation {
	op := &PendingOperation{
		MessageID: messageID,
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}

	h.mu.Lock()
	h.pendingOps[messageID] = op
	h.mu.Unlock()

	return op
}

// Unregister removes a pending operation from tracking.
// This should be called when an operation completes (either normally
// or due to cancellation) to clean up resources.
func (h *AbandonHandler) Unregister(messageID int) {
	h.mu.Lock()
	if op, exists := h.pendingOps[messageID]; exists {
		// Close the Done channel to signal completion
		select {
		case <-op.Done:
			// Already closed
		default:
			close(op.Done)
		}
		delete(h.pendingOps, messageID)
	}
	h.mu.Unlock()
}

// IsPending returns true if an operation with the given message ID is pending.
func (h *AbandonHandler) IsPending(messageID int) bool {
	h.mu.RLock()
	_, exists := h.pendingOps[messageID]
	h.mu.RUnlock()
	return exists
}

// PendingCount returns the number of pending operations.
func (h *AbandonHandler) PendingCount() int {
	h.mu.RLock()
	count := len(h.pendingOps)
	h.mu.RUnlock()
	return count
}

// CancelAll cancels all pending operations.
// This is useful during server shutdown.
func (h *AbandonHandler) CancelAll() {
	h.mu.Lock()
	for _, op := range h.pendingOps {
		op.Cancel()
		select {
		case <-op.Done:
			// Already closed
		default:
			close(op.Done)
		}
	}
	// Clear the map
	h.pendingOps = make(map[int]*PendingOperation)
	h.mu.Unlock()
}

// GetOperation returns the pending operation for the given message ID,
// or nil if no such operation exists.
func (h *AbandonHandler) GetOperation(messageID int) *PendingOperation {
	h.mu.RLock()
	op := h.pendingOps[messageID]
	h.mu.RUnlock()
	return op
}
