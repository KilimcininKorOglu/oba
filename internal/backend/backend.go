// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/filter"
	"github.com/KilimcininKorOglu/oba/internal/password"
	"github.com/KilimcininKorOglu/oba/internal/schema"
	"github.com/KilimcininKorOglu/oba/internal/server"
	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/stream"
)

// Backend errors.
var (
	// ErrInvalidCredentials is returned when authentication fails.
	ErrInvalidCredentials = errors.New("backend: invalid credentials")
	// ErrEntryNotFound is returned when an entry is not found.
	ErrEntryNotFound = errors.New("backend: entry not found")
	// ErrEntryExists is returned when an entry already exists.
	ErrEntryExists = errors.New("backend: entry already exists")
	// ErrInvalidDN is returned when a DN is invalid.
	ErrInvalidDN = errors.New("backend: invalid DN")
	// ErrInvalidEntry is returned when an entry is invalid.
	ErrInvalidEntry = errors.New("backend: invalid entry")
	// ErrNoPassword is returned when an entry has no password attribute.
	ErrNoPassword = errors.New("backend: no password attribute")
	// ErrStorageError is returned when a storage operation fails.
	ErrStorageError = errors.New("backend: storage error")
	// ErrNotAllowedOnNonLeaf is returned when trying to delete an entry with children.
	ErrNotAllowedOnNonLeaf = errors.New("backend: operation not allowed on non-leaf entry")
)

// PasswordAttribute is the standard LDAP attribute name for user passwords.
const PasswordAttribute = "userpassword"

// Backend defines the interface for LDAP backend operations.
// It wraps the storage engine and provides LDAP-specific functionality.
type Backend interface {
	// Bind authenticates a user with the given DN and password.
	// Returns nil if authentication succeeds, or an error otherwise.
	Bind(dn, password string) error

	// Search searches for entries matching the given criteria.
	// baseDN is the base distinguished name for the search.
	// scope is the search scope (base, one-level, or subtree).
	// f is the search filter.
	// Returns matching entries or an error.
	Search(baseDN string, scope int, f *filter.Filter) ([]*Entry, error)

	// Add adds a new entry to the directory.
	// Returns an error if the entry already exists or is invalid.
	Add(entry *Entry) error

	// AddWithBindDN adds a new entry to the directory with operational attributes.
	// The bindDN is used to set creatorsName and modifiersName.
	// Returns an error if the entry already exists or is invalid.
	AddWithBindDN(entry *Entry, bindDN string) error

	// Delete removes an entry from the directory.
	// Returns an error if the entry does not exist.
	Delete(dn string) error

	// HasChildren returns true if the entry has child entries.
	HasChildren(dn string) (bool, error)

	// Modify modifies an existing entry.
	// Returns an error if the entry does not exist or the modifications are invalid.
	Modify(dn string, changes []Modification) error

	// ModifyWithBindDN modifies an existing entry with operational attributes.
	// The bindDN is used to set modifiersName.
	// Returns an error if the entry does not exist or the modifications are invalid.
	ModifyWithBindDN(dn string, changes []Modification, bindDN string) error
}

// ObaBackend implements the Backend interface using the ObaDB storage engine.
type ObaBackend struct {
	engine       storage.StorageEngine
	schema       *schema.Schema
	rootDN       string
	rootPW       string
	changeStream *stream.Broker

	// Security settings (hot-reloadable)
	rateLimitEnabled  bool
	rateLimitAttempts int
	rateLimitDuration time.Duration
	passwordPolicy    *password.Policy
	accountLockouts   map[string]*password.AccountLockout
	securityMu        sync.RWMutex
}

// NewBackend creates a new ObaBackend with the given storage engine and configuration.
func NewBackend(engine storage.StorageEngine, cfg *config.Config) *ObaBackend {
	b := &ObaBackend{
		engine:          engine,
		changeStream:    stream.NewBroker(),
		accountLockouts: make(map[string]*password.AccountLockout),
	}

	if cfg != nil {
		b.rootDN = normalizeDN(cfg.Directory.RootDN)
		b.rootPW = cfg.Directory.RootPassword

		// Initialize security settings
		b.rateLimitEnabled = cfg.Security.RateLimit.Enabled
		b.rateLimitAttempts = cfg.Security.RateLimit.MaxAttempts
		b.rateLimitDuration = cfg.Security.RateLimit.LockoutDuration

		if cfg.Security.PasswordPolicy.Enabled {
			b.passwordPolicy = &password.Policy{
				Enabled:          cfg.Security.PasswordPolicy.Enabled,
				MinLength:        cfg.Security.PasswordPolicy.MinLength,
				RequireUppercase: cfg.Security.PasswordPolicy.RequireUppercase,
				RequireLowercase: cfg.Security.PasswordPolicy.RequireLowercase,
				RequireDigit:     cfg.Security.PasswordPolicy.RequireDigit,
				RequireSpecial:   cfg.Security.PasswordPolicy.RequireSpecial,
				MaxAge:           cfg.Security.PasswordPolicy.MaxAge,
				HistoryCount:     cfg.Security.PasswordPolicy.HistoryCount,
			}
		}
	}

	return b
}

// SetSchema sets the schema for entry validation.
func (b *ObaBackend) SetSchema(s *schema.Schema) {
	b.schema = s
}

// Bind authenticates a user with the given DN and password.
// It first checks for root DN (admin) bind, then looks up the entry
// in storage and verifies the password hash.
func (b *ObaBackend) Bind(dn, password string) error {
	if dn == "" {
		// Anonymous bind - always succeeds
		return nil
	}

	normalizedDN := normalizeDN(dn)

	// Check for root DN (admin) bind
	if b.rootDN != "" && normalizedDN == b.rootDN {
		return b.verifyRootPassword(password)
	}

	// Look up entry in storage
	entry, err := b.getEntry(normalizedDN)
	if err != nil {
		if err == ErrEntryNotFound {
			return ErrInvalidCredentials
		}
		return err
	}

	// Verify password
	return b.verifyEntryPassword(entry, password)
}

// verifyRootPassword verifies the password against the root password.
func (b *ObaBackend) verifyRootPassword(password string) error {
	if b.rootPW == "" {
		return ErrInvalidCredentials
	}

	err := server.VerifyPassword(password, b.rootPW)
	if err != nil {
		return ErrInvalidCredentials
	}

	return nil
}

// verifyEntryPassword verifies the password against the entry's userPassword attribute.
func (b *ObaBackend) verifyEntryPassword(entry *Entry, password string) error {
	passwords := entry.GetAttribute(PasswordAttribute)
	if len(passwords) == 0 {
		return ErrNoPassword
	}

	// Try each stored password (there may be multiple)
	for _, storedPassword := range passwords {
		err := server.VerifyPassword(password, storedPassword)
		if err == nil {
			return nil
		}
	}

	return ErrInvalidCredentials
}

// Search searches for entries matching the given criteria.
func (b *ObaBackend) Search(baseDN string, scope int, f *filter.Filter) ([]*Entry, error) {
	normalizedBaseDN := normalizeDN(baseDN)

	// Start a read transaction
	txn, err := b.engine.Begin()
	if err != nil {
		return nil, wrapStorageError(err)
	}
	defer b.engine.Rollback(txn)

	// Convert scope to storage.Scope
	storageScope := storage.Scope(scope)

	// Create filter evaluator
	evaluator := filter.NewEvaluator(b.schema)

	var iter storage.Iterator
	if f != nil {
		// Create a filter matcher wrapper
		matcher := &filterMatcherWrapper{filter: f, evaluator: evaluator}
		iter = b.engine.SearchByFilter(txn, normalizedBaseDN, matcher)
	} else {
		iter = b.engine.SearchByDN(txn, normalizedBaseDN, storageScope)
	}
	defer iter.Close()

	var results []*Entry
	for iter.Next() {
		storageEntry := iter.Entry()
		if storageEntry == nil {
			continue
		}

		// Convert storage entry to backend entry
		entry := convertFromStorageEntry(storageEntry)
		results = append(results, entry)
	}

	if err := iter.Error(); err != nil {
		return nil, wrapStorageError(err)
	}

	return results, nil
}

// Add adds a new entry to the directory.
// This is a convenience method that calls AddWithBindDN with an empty bindDN.
func (b *ObaBackend) Add(entry *Entry) error {
	return b.AddWithBindDN(entry, "")
}

// AddWithBindDN adds a new entry to the directory with operational attributes.
// The bindDN is used to set creatorsName and modifiersName.
func (b *ObaBackend) AddWithBindDN(entry *Entry, bindDN string) error {
	if entry == nil || entry.DN == "" {
		return ErrInvalidEntry
	}

	normalizedDN := normalizeDN(entry.DN)
	entry.DN = normalizedDN

	// Set operational attributes for add operation
	SetOperationalAttrs(entry, OpAdd, bindDN)

	// Validate entry against schema if available
	if b.schema != nil {
		if err := b.validateEntry(entry); err != nil {
			return err
		}
	}

	// Start a transaction
	txn, err := b.engine.Begin()
	if err != nil {
		return wrapStorageError(err)
	}

	// Check if entry already exists
	_, err = b.engine.Get(txn, normalizedDN)
	if err == nil {
		b.engine.Rollback(txn)
		return ErrEntryExists
	}

	// Convert to storage entry
	storageEntry := convertToStorageEntry(entry)

	// Put the entry
	if err := b.engine.Put(txn, storageEntry); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Commit the transaction
	if err := b.engine.Commit(txn); err != nil {
		return wrapStorageError(err)
	}

	// Emit change event after successful commit
	b.emitChange(stream.OpInsert, normalizedDN, storageEntry)

	return nil
}

// Delete removes an entry from the directory.
func (b *ObaBackend) Delete(dn string) error {
	if dn == "" {
		return ErrInvalidDN
	}

	normalizedDN := normalizeDN(dn)

	// Start a transaction
	txn, err := b.engine.Begin()
	if err != nil {
		return wrapStorageError(err)
	}

	// Check if entry exists
	_, err = b.engine.Get(txn, normalizedDN)
	if err != nil {
		b.engine.Rollback(txn)
		return ErrEntryNotFound
	}

	// Delete the entry
	if err := b.engine.Delete(txn, normalizedDN); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Commit the transaction
	if err := b.engine.Commit(txn); err != nil {
		return wrapStorageError(err)
	}

	// Emit change event after successful commit
	b.emitChange(stream.OpDelete, normalizedDN, nil)

	return nil
}

// HasChildren returns true if the entry has child entries.
func (b *ObaBackend) HasChildren(dn string) (bool, error) {
	if dn == "" {
		return false, ErrInvalidDN
	}

	normalizedDN := normalizeDN(dn)

	// Start a read transaction
	txn, err := b.engine.Begin()
	if err != nil {
		return false, wrapStorageError(err)
	}
	defer b.engine.Rollback(txn)

	// Check if entry has children
	return b.engine.HasChildren(txn, normalizedDN)
}

// Modify modifies an existing entry.
// This is a convenience method that calls ModifyWithBindDN with an empty bindDN.
func (b *ObaBackend) Modify(dn string, changes []Modification) error {
	return b.ModifyWithBindDN(dn, changes, "")
}

// ModifyWithBindDN modifies an existing entry with operational attributes.
// The bindDN is used to set modifiersName.
func (b *ObaBackend) ModifyWithBindDN(dn string, changes []Modification, bindDN string) error {
	if dn == "" {
		return ErrInvalidDN
	}

	if len(changes) == 0 {
		return nil
	}

	normalizedDN := normalizeDN(dn)

	// Start a transaction
	txn, err := b.engine.Begin()
	if err != nil {
		return wrapStorageError(err)
	}

	// Get the existing entry
	storageEntry, err := b.engine.Get(txn, normalizedDN)
	if err != nil {
		b.engine.Rollback(txn)
		return ErrEntryNotFound
	}

	// Convert to backend entry for modification
	entry := convertFromStorageEntry(storageEntry)

	// Apply modifications
	for _, mod := range changes {
		attrName := strings.ToLower(mod.Attribute)

		switch mod.Type {
		case ModAdd:
			for _, value := range mod.Values {
				entry.AddAttributeValue(attrName, value)
			}

		case ModDelete:
			if len(mod.Values) == 0 {
				// Delete entire attribute
				entry.DeleteAttribute(attrName)
			} else {
				// Delete specific values
				for _, value := range mod.Values {
					entry.DeleteAttributeValue(attrName, value)
				}
			}

		case ModReplace:
			if len(mod.Values) == 0 {
				// Replace with empty = delete
				entry.DeleteAttribute(attrName)
			} else {
				entry.SetAttribute(attrName, mod.Values...)
			}
		}
	}

	// Set operational attributes for modify operation
	SetOperationalAttrs(entry, OpModify, bindDN)

	// Validate modified entry against schema if available
	if b.schema != nil {
		if err := b.validateEntry(entry); err != nil {
			b.engine.Rollback(txn)
			return err
		}
	}

	// Convert back to storage entry
	modifiedStorageEntry := convertToStorageEntry(entry)

	// Put the modified entry
	if err := b.engine.Put(txn, modifiedStorageEntry); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Commit the transaction
	if err := b.engine.Commit(txn); err != nil {
		return wrapStorageError(err)
	}

	// Emit change event after successful commit
	b.emitChange(stream.OpUpdate, normalizedDN, modifiedStorageEntry)

	return nil
}

// getEntry retrieves an entry by DN.
func (b *ObaBackend) getEntry(dn string) (*Entry, error) {
	txn, err := b.engine.Begin()
	if err != nil {
		return nil, wrapStorageError(err)
	}
	defer b.engine.Rollback(txn)

	storageEntry, err := b.engine.Get(txn, dn)
	if err != nil {
		return nil, ErrEntryNotFound
	}

	return convertFromStorageEntry(storageEntry), nil
}

// validateEntry validates an entry against the schema.
func (b *ObaBackend) validateEntry(entry *Entry) error {
	if b.schema == nil {
		return nil
	}

	// Convert to schema entry for validation
	schemaEntry := &schema.Entry{
		DN:         entry.DN,
		Attributes: make(map[string][][]byte),
	}

	for name, values := range entry.Attributes {
		byteValues := make([][]byte, len(values))
		for i, v := range values {
			byteValues[i] = []byte(v)
		}
		schemaEntry.Attributes[name] = byteValues
	}

	validator := schema.NewValidator(b.schema)
	return validator.ValidateEntry(schemaEntry)
}

// convertToStorageEntry converts a backend Entry to a storage Entry.
func convertToStorageEntry(entry *Entry) *storage.Entry {
	storageEntry := storage.NewEntry(entry.DN)

	for name, values := range entry.Attributes {
		byteValues := make([][]byte, len(values))
		for i, v := range values {
			byteValues[i] = []byte(v)
		}
		storageEntry.SetAttribute(name, byteValues)
	}

	return storageEntry
}

// convertFromStorageEntry converts a storage Entry to a backend Entry.
func convertFromStorageEntry(storageEntry *storage.Entry) *Entry {
	entry := NewEntry(storageEntry.DN)

	for name, values := range storageEntry.Attributes {
		stringValues := make([]string, len(values))
		for i, v := range values {
			stringValues[i] = string(v)
		}
		entry.Attributes[name] = stringValues
	}

	return entry
}

// convertToFilterEntry converts a backend Entry to a filter Entry.
func convertToFilterEntry(entry *Entry) *filter.Entry {
	filterEntry := filter.NewEntry(entry.DN)

	for name, values := range entry.Attributes {
		byteValues := make([][]byte, len(values))
		for i, v := range values {
			byteValues[i] = []byte(v)
		}
		filterEntry.SetAttribute(name, byteValues...)
	}

	return filterEntry
}

// filterMatcherWrapper wraps a filter.Filter to implement storage.FilterMatcher.
type filterMatcherWrapper struct {
	filter    *filter.Filter
	evaluator *filter.Evaluator
}

// Match implements storage.FilterMatcher.
func (w *filterMatcherWrapper) Match(entry *storage.Entry) bool {
	if w.filter == nil || entry == nil {
		return true
	}

	// Convert storage entry to filter entry
	filterEntry := filter.NewEntry(entry.DN)
	for name, values := range entry.Attributes {
		filterEntry.SetAttribute(name, values...)
	}

	return w.evaluator.Evaluate(w.filter, filterEntry)
}

// normalizeDN normalizes a DN for consistent storage and lookup.
func normalizeDN(dn string) string {
	return strings.TrimSpace(strings.ToLower(dn))
}

// wrapStorageError wraps a storage error with a backend error.
func wrapStorageError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New("backend: " + err.Error())
}

// Watch creates a new change stream subscription with the given filter.
// Returns a Subscriber that receives matching events on its Channel.
func (b *ObaBackend) Watch(filter stream.WatchFilter) *stream.Subscriber {
	return b.changeStream.Subscribe(filter)
}

// WatchWithResume creates a subscription and replays events from the given token.
// Returns stream.ErrTokenTooOld if the token is older than the oldest event in the replay buffer.
func (b *ObaBackend) WatchWithResume(filter stream.WatchFilter, resumeToken uint64) (*stream.Subscriber, error) {
	return b.changeStream.SubscribeWithResume(filter, resumeToken)
}

// Unwatch removes a subscription by ID.
func (b *ObaBackend) Unwatch(id stream.SubscriberID) {
	b.changeStream.Unsubscribe(id)
}

// ChangeStreamStats returns statistics about the change stream broker.
func (b *ObaBackend) ChangeStreamStats() stream.BrokerStats {
	return b.changeStream.Stats()
}

// emitChange publishes a change event to all matching subscribers.
func (b *ObaBackend) emitChange(op stream.OperationType, dn string, entry *storage.Entry) {
	if !b.changeStream.HasSubscribers() {
		return
	}
	b.changeStream.Publish(stream.ChangeEvent{
		Operation: op,
		DN:        dn,
		Entry:     entry,
	})
}

// Close closes the backend and releases resources.
func (b *ObaBackend) Close() {
	if b.changeStream != nil {
		b.changeStream.Close()
	}
}

// SearchByDN searches for entries by DN with the given scope.
// Returns an iterator over matching entries.
func (b *ObaBackend) SearchByDN(baseDN string, scope storage.Scope) storage.Iterator {
	txn, err := b.engine.Begin()
	if err != nil {
		return &errorIterator{err: wrapStorageError(err)}
	}
	// Note: Transaction will be rolled back when iterator is closed
	return &backendIterator{
		engine: b.engine,
		txn:    txn,
		iter:   b.engine.SearchByDN(txn, baseDN, scope),
	}
}

// backendIterator wraps a storage iterator with transaction management.
type backendIterator struct {
	engine storage.StorageEngine
	txn    interface{}
	iter   storage.Iterator
}

func (it *backendIterator) Next() bool {
	return it.iter.Next()
}

func (it *backendIterator) Entry() *storage.Entry {
	return it.iter.Entry()
}

func (it *backendIterator) Error() error {
	return it.iter.Error()
}

func (it *backendIterator) Close() {
	it.iter.Close()
	if it.engine != nil && it.txn != nil {
		it.engine.Rollback(it.txn)
	}
}

// errorIterator returns an error on first access.
type errorIterator struct {
	err error
}

func (it *errorIterator) Next() bool            { return false }
func (it *errorIterator) Entry() *storage.Entry { return nil }
func (it *errorIterator) Error() error          { return it.err }
func (it *errorIterator) Close()                {}

// Ensure ObaBackend implements Backend interface.
var _ Backend = (*ObaBackend)(nil)

// SetRateLimitConfig updates rate limit settings at runtime.
func (b *ObaBackend) SetRateLimitConfig(enabled bool, maxAttempts int, lockoutDuration time.Duration) {
	b.securityMu.Lock()
	defer b.securityMu.Unlock()

	b.rateLimitEnabled = enabled
	b.rateLimitAttempts = maxAttempts
	b.rateLimitDuration = lockoutDuration

	// Update existing lockouts with new settings
	for _, lockout := range b.accountLockouts {
		lockout.SetMaxFailures(maxAttempts)
		lockout.SetLockoutDuration(lockoutDuration)
	}
}

// GetRateLimitConfig returns the current rate limit configuration.
func (b *ObaBackend) GetRateLimitConfig() (enabled bool, maxAttempts int, lockoutDuration time.Duration) {
	b.securityMu.RLock()
	defer b.securityMu.RUnlock()
	return b.rateLimitEnabled, b.rateLimitAttempts, b.rateLimitDuration
}

// SetPasswordPolicy updates password policy settings at runtime.
func (b *ObaBackend) SetPasswordPolicy(policy *password.Policy) {
	b.securityMu.Lock()
	defer b.securityMu.Unlock()
	b.passwordPolicy = policy
}

// GetPasswordPolicy returns the current password policy.
func (b *ObaBackend) GetPasswordPolicy() *password.Policy {
	b.securityMu.RLock()
	defer b.securityMu.RUnlock()
	return b.passwordPolicy
}

// GetAccountLockout returns the lockout state for a DN.
func (b *ObaBackend) GetAccountLockout(dn string) *password.AccountLockout {
	b.securityMu.Lock()
	defer b.securityMu.Unlock()

	normalizedDN := normalizeDN(dn)
	lockout, exists := b.accountLockouts[normalizedDN]
	if !exists {
		lockout = password.NewAccountLockout(b.rateLimitAttempts, b.rateLimitDuration, 0)
		b.accountLockouts[normalizedDN] = lockout
	}
	return lockout
}

// IsAccountLocked checks if an account is locked.
func (b *ObaBackend) IsAccountLocked(dn string) bool {
	if !b.rateLimitEnabled {
		return false
	}
	lockout := b.GetAccountLockout(dn)
	return lockout.IsLocked()
}

// RecordAuthFailure records a failed authentication attempt.
func (b *ObaBackend) RecordAuthFailure(dn string) {
	if !b.rateLimitEnabled {
		return
	}
	lockout := b.GetAccountLockout(dn)
	lockout.RecordFailure()
}

// RecordAuthSuccess records a successful authentication.
func (b *ObaBackend) RecordAuthSuccess(dn string) {
	if !b.rateLimitEnabled {
		return
	}
	lockout := b.GetAccountLockout(dn)
	lockout.RecordSuccess()
}

// UnlockAccount manually unlocks an account.
func (b *ObaBackend) UnlockAccount(dn string) {
	b.securityMu.Lock()
	defer b.securityMu.Unlock()

	normalizedDN := normalizeDN(dn)
	if lockout, exists := b.accountLockouts[normalizedDN]; exists {
		lockout.Unlock()
	}
}
