// Package server provides the LDAP server implementation.
package server

import (
	"context"
	"sync"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/stream"
)

// Persistent Search Control OID (draft-ietf-ldapext-psearch)
const (
	PersistentSearchOID        = "2.16.840.1.113730.3.4.3"
	EntryChangeNotificationOID = "2.16.840.1.113730.3.4.7"
)

// Change types for Persistent Search
const (
	ChangeTypeAdd    = 1
	ChangeTypeDelete = 2
	ChangeTypeModify = 4
	ChangeTypeModDN  = 8
)

// PersistentSearchControl represents the Persistent Search Control.
//
//	PersistentSearch ::= SEQUENCE {
//	    changeTypes INTEGER,
//	    changesOnly BOOLEAN,
//	    returnECs   BOOLEAN
//	}
type PersistentSearchControl struct {
	ChangeTypes int  // Bitmask: 1=add, 2=delete, 4=modify, 8=modDN
	ChangesOnly bool // If true, only return changes (not initial entries)
	ReturnECs   bool // If true, include Entry Change Notification control
	Criticality bool
}

// ParsePersistentSearchControl parses a Persistent Search Control from an LDAP Control.
func ParsePersistentSearchControl(ctrl ldap.Control) (*PersistentSearchControl, error) {
	if ctrl.OID != PersistentSearchOID {
		return nil, nil
	}

	psc := &PersistentSearchControl{
		Criticality: ctrl.Criticality,
		ChangeTypes: ChangeTypeAdd | ChangeTypeDelete | ChangeTypeModify | ChangeTypeModDN, // Default: all
		ChangesOnly: false,
		ReturnECs:   true,
	}

	if len(ctrl.Value) == 0 {
		return psc, nil
	}

	decoder := ber.NewBERDecoder(ctrl.Value)

	// Read SEQUENCE
	_, err := decoder.ExpectSequence()
	if err != nil {
		return nil, err
	}

	// Read changeTypes (INTEGER)
	changeTypes, err := decoder.ReadInteger()
	if err != nil {
		return nil, err
	}
	psc.ChangeTypes = int(changeTypes)

	// Read changesOnly (BOOLEAN)
	changesOnly, err := decoder.ReadBoolean()
	if err != nil {
		return nil, err
	}
	psc.ChangesOnly = changesOnly

	// Read returnECs (BOOLEAN)
	returnECs, err := decoder.ReadBoolean()
	if err != nil {
		return nil, err
	}
	psc.ReturnECs = returnECs

	return psc, nil
}

// FindPersistentSearchControl searches for a Persistent Search Control in controls.
func FindPersistentSearchControl(controls []ldap.Control) (*PersistentSearchControl, error) {
	for _, ctrl := range controls {
		if ctrl.OID == PersistentSearchOID {
			return ParsePersistentSearchControl(ctrl)
		}
	}
	return nil, nil
}

// EntryChangeNotification represents the Entry Change Notification control.
//
//	EntryChangeNotification ::= SEQUENCE {
//	    changeType ENUMERATED { add(1), delete(2), modify(4), modDN(8) },
//	    previousDN LDAPDN OPTIONAL,
//	    changeNumber INTEGER OPTIONAL
//	}
type EntryChangeNotification struct {
	ChangeType   int
	PreviousDN   string // Only for modDN
	ChangeNumber uint64
}

// Encode encodes the EntryChangeNotification to BER format.
func (ecn *EntryChangeNotification) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	seqPos := encoder.BeginSequence()

	// Write changeType (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(ecn.ChangeType)); err != nil {
		return nil, err
	}

	// Write previousDN if present (for modDN)
	if ecn.PreviousDN != "" {
		if err := encoder.WriteOctetString([]byte(ecn.PreviousDN)); err != nil {
			return nil, err
		}
	}

	// Write changeNumber if non-zero
	if ecn.ChangeNumber > 0 {
		if err := encoder.WriteInteger(int64(ecn.ChangeNumber)); err != nil {
			return nil, err
		}
	}

	if err := encoder.EndSequence(seqPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// ToLDAPControl converts EntryChangeNotification to an ldap.Control.
func (ecn *EntryChangeNotification) ToLDAPControl() (ldap.Control, error) {
	value, err := ecn.Encode()
	if err != nil {
		return ldap.Control{}, err
	}

	return ldap.Control{
		OID:         EntryChangeNotificationOID,
		Criticality: false,
		Value:       value,
	}, nil
}

// PersistentSearchBackend defines the interface for persistent search operations.
type PersistentSearchBackend interface {
	// Watch creates a change stream subscription.
	Watch(filter stream.WatchFilter) *stream.Subscriber
	// Unwatch removes a subscription.
	Unwatch(id stream.SubscriberID)
	// GetEntry retrieves an entry by DN.
	GetEntry(dn string) (*storage.Entry, error)
	// SearchByDN searches for entries by DN with the given scope.
	SearchByDN(baseDN string, scope storage.Scope) storage.Iterator
}

// PersistentSearchHandler handles persistent search requests.
type PersistentSearchHandler struct {
	backend  PersistentSearchBackend
	mu       sync.Mutex
	sessions map[*Connection]*persistentSearchSession
}

type persistentSearchSession struct {
	subscriber *stream.Subscriber
	cancel     context.CancelFunc
}

// NewPersistentSearchHandler creates a new persistent search handler.
func NewPersistentSearchHandler(backend PersistentSearchBackend) *PersistentSearchHandler {
	return &PersistentSearchHandler{
		backend:  backend,
		sessions: make(map[*Connection]*persistentSearchSession),
	}
}

// Handle processes a persistent search request.
// This method blocks until the connection is closed or an error occurs.
func (h *PersistentSearchHandler) Handle(
	conn *Connection,
	req *ldap.SearchRequest,
	ctrl *PersistentSearchControl,
	messageID int,
) {
	if h.backend == nil {
		h.sendSearchDone(conn, messageID, ldap.ResultUnwillingToPerform, "persistent search not configured")
		return
	}

	// Create watch filter based on search request
	watchFilter := stream.WatchFilter{
		BaseDN: req.BaseObject,
		Scope:  int(req.Scope),
	}

	// Map change types to operations
	var ops []stream.OperationType
	if ctrl.ChangeTypes&ChangeTypeAdd != 0 {
		ops = append(ops, stream.OpInsert)
	}
	if ctrl.ChangeTypes&ChangeTypeDelete != 0 {
		ops = append(ops, stream.OpDelete)
	}
	if ctrl.ChangeTypes&ChangeTypeModify != 0 {
		ops = append(ops, stream.OpUpdate)
	}
	if ctrl.ChangeTypes&ChangeTypeModDN != 0 {
		ops = append(ops, stream.OpModifyDN)
	}
	watchFilter.Operations = ops

	// Subscribe to changes
	sub := h.backend.Watch(watchFilter)
	if sub == nil {
		h.sendSearchDone(conn, messageID, ldap.ResultUnwillingToPerform, "failed to subscribe")
		return
	}

	// Create cancellation context
	ctx, cancel := context.WithCancel(context.Background())

	// Store session
	h.mu.Lock()
	h.sessions[conn] = &persistentSearchSession{
		subscriber: sub,
		cancel:     cancel,
	}
	h.mu.Unlock()

	// Cleanup on exit
	defer func() {
		h.mu.Lock()
		delete(h.sessions, conn)
		h.mu.Unlock()
		h.backend.Unwatch(sub.ID)
		cancel()
	}()

	// Send initial results if not changesOnly
	if !ctrl.ChangesOnly {
		if err := h.sendInitialResults(conn, req, messageID); err != nil {
			return
		}
	}

	// Stream changes
	for {
		select {
		case event, ok := <-sub.Channel:
			if !ok {
				// Channel closed
				h.sendSearchDone(conn, messageID, ldap.ResultSuccess, "")
				return
			}

			// Send the change as a search result entry
			if err := h.sendChangeEvent(conn, messageID, &event, ctrl.ReturnECs); err != nil {
				return
			}

		case <-ctx.Done():
			h.sendSearchDone(conn, messageID, ldap.ResultSuccess, "")
			return
		}
	}
}

// sendInitialResults sends the initial search results before streaming changes.
func (h *PersistentSearchHandler) sendInitialResults(conn *Connection, req *ldap.SearchRequest, messageID int) error {
	iter := h.backend.SearchByDN(req.BaseObject, storage.Scope(req.Scope))
	if iter == nil {
		return nil
	}
	defer iter.Close()

	count := 0
	for iter.Next() {
		entry := iter.Entry()
		if entry == nil {
			continue
		}

		// Check size limit
		if req.SizeLimit > 0 && count >= int(req.SizeLimit) {
			break
		}

		// Build and send search entry
		searchEntry := h.buildSearchEntry(entry, req.Attributes, req.TypesOnly)
		msg := h.createSearchEntryResponse(messageID, searchEntry, nil)
		if err := conn.WriteMessage(msg); err != nil {
			return err
		}
		count++
	}

	return iter.Error()
}

// sendChangeEvent sends a change event as a search result entry.
func (h *PersistentSearchHandler) sendChangeEvent(
	conn *Connection,
	messageID int,
	event *stream.ChangeEvent,
	returnECs bool,
) error {
	// For delete operations, we can't send the entry (it's gone)
	if event.Operation == stream.OpDelete {
		// Send a minimal entry with just the DN
		searchEntry := &SearchEntry{DN: event.DN}
		var ecn *EntryChangeNotification
		if returnECs {
			ecn = &EntryChangeNotification{
				ChangeType:   ChangeTypeDelete,
				ChangeNumber: event.Token,
			}
		}
		msg := h.createSearchEntryResponse(messageID, searchEntry, ecn)
		return conn.WriteMessage(msg)
	}

	// For other operations, send the entry
	if event.Entry == nil {
		return nil
	}

	searchEntry := h.buildSearchEntry(event.Entry, nil, false)

	var ecn *EntryChangeNotification
	if returnECs {
		ecn = &EntryChangeNotification{
			ChangeNumber: event.Token,
		}
		switch event.Operation {
		case stream.OpInsert:
			ecn.ChangeType = ChangeTypeAdd
		case stream.OpUpdate:
			ecn.ChangeType = ChangeTypeModify
		case stream.OpModifyDN:
			ecn.ChangeType = ChangeTypeModDN
			ecn.PreviousDN = event.OldDN
		}
	}

	msg := h.createSearchEntryResponse(messageID, searchEntry, ecn)
	return conn.WriteMessage(msg)
}

// buildSearchEntry builds a SearchEntry from a storage.Entry.
func (h *PersistentSearchHandler) buildSearchEntry(entry *storage.Entry, requestedAttrs []string, typesOnly bool) *SearchEntry {
	searchEntry := &SearchEntry{DN: entry.DN}

	// Select attributes
	attrs := entry.Attributes
	if len(requestedAttrs) > 0 {
		attrs = make(map[string][][]byte)
		for _, name := range requestedAttrs {
			if values, ok := entry.Attributes[name]; ok {
				attrs[name] = values
			}
		}
	}

	for name, values := range attrs {
		attr := ldap.Attribute{Type: name}
		if !typesOnly {
			attr.Values = values
		}
		searchEntry.Attributes = append(searchEntry.Attributes, attr)
	}

	return searchEntry
}

// createSearchEntryResponse creates a SearchResultEntry message with optional ECN control.
func (h *PersistentSearchHandler) createSearchEntryResponse(
	messageID int,
	entry *SearchEntry,
	ecn *EntryChangeNotification,
) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(256)

	// Write objectName (LDAPDN)
	if err := encoder.WriteOctetString([]byte(entry.DN)); err != nil {
		return nil
	}

	// Write attributes (SEQUENCE OF PartialAttribute)
	attrListPos := encoder.BeginSequence()
	for _, attr := range entry.Attributes {
		attrPos := encoder.BeginSequence()
		if err := encoder.WriteOctetString([]byte(attr.Type)); err != nil {
			return nil
		}
		valSetPos := encoder.BeginSet()
		for _, value := range attr.Values {
			if err := encoder.WriteOctetString(value); err != nil {
				return nil
			}
		}
		encoder.EndSet(valSetPos)
		encoder.EndSequence(attrPos)
	}
	encoder.EndSequence(attrListPos)

	msg := &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchResultEntry,
			Data: encoder.Bytes(),
		},
	}

	// Add ECN control if requested
	if ecn != nil {
		ctrl, err := ecn.ToLDAPControl()
		if err == nil {
			msg.Controls = append(msg.Controls, ctrl)
		}
	}

	return msg
}

// sendSearchDone sends a SearchResultDone message.
func (h *PersistentSearchHandler) sendSearchDone(conn *Connection, messageID int, resultCode ldap.ResultCode, message string) {
	encoder := ber.NewBEREncoder(128)
	encoder.WriteEnumerated(int64(resultCode))
	encoder.WriteOctetString([]byte(""))
	encoder.WriteOctetString([]byte(message))

	msg := &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchResultDone,
			Data: encoder.Bytes(),
		},
	}
	conn.WriteMessage(msg)
}

// CancelSession cancels a persistent search session for a connection.
func (h *PersistentSearchHandler) CancelSession(conn *Connection) {
	h.mu.Lock()
	session, ok := h.sessions[conn]
	h.mu.Unlock()

	if ok && session != nil {
		session.cancel()
	}
}

// ActiveSessions returns the number of active persistent search sessions.
func (h *PersistentSearchHandler) ActiveSessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}
