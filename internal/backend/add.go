// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/radix"
)

// ErrNoParent is returned when the parent entry does not exist.
var ErrNoParent = ErrEntryNotFound

// ErrObjectClassRequired is returned when an entry is missing the objectClass attribute.
var ErrObjectClassRequired = ErrInvalidEntry

// AddEntry adds a new entry to the directory with full validation.
// This method accepts a storage.Entry and performs the following checks:
// 1. Validates the DN is not empty
// 2. Checks if the entry already exists (returns ErrEntryExists)
// 3. Checks if the parent entry exists (returns ErrNoParent)
// 4. Validates that objectClass attribute is present
// 5. Validates entry against schema if available
// 6. Inserts the entry into storage within a transaction
func (b *ObaBackend) AddEntry(entry *storage.Entry) error {
	if entry == nil || entry.DN == "" {
		return ErrInvalidEntry
	}

	normalizedDN := normalizeDN(entry.DN)

	// Validate objectClass is present
	if !hasStorageObjectClass(entry) {
		return ErrObjectClassRequired
	}

	// Convert to backend entry for schema validation
	backendEntry := convertFromStorageEntry(entry)
	backendEntry.DN = normalizedDN

	// Validate entry against schema if available
	if b.schema != nil {
		if err := b.validateEntry(backendEntry); err != nil {
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

	// Check if parent entry exists (unless this is a root entry)
	parentDN, err := radix.GetParentDN(normalizedDN)
	if err != nil {
		b.engine.Rollback(txn)
		return ErrInvalidDN
	}

	if parentDN != "" {
		_, err = b.engine.Get(txn, parentDN)
		if err != nil {
			b.engine.Rollback(txn)
			return ErrNoParent
		}
	}

	// Create storage entry with normalized DN
	storageEntry := entry.Clone()
	storageEntry.DN = normalizedDN

	// Put the entry
	if err := b.engine.Put(txn, storageEntry); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Commit the transaction
	if err := b.engine.Commit(txn); err != nil {
		return wrapStorageError(err)
	}

	return nil
}

// GetEntry retrieves an entry by its DN.
// Returns nil if the entry does not exist.
func (b *ObaBackend) GetEntry(dn string) (*storage.Entry, error) {
	if dn == "" {
		return nil, ErrInvalidDN
	}

	normalizedDN := normalizeDN(dn)

	txn, err := b.engine.Begin()
	if err != nil {
		return nil, wrapStorageError(err)
	}
	defer b.engine.Rollback(txn)

	storageEntry, err := b.engine.Get(txn, normalizedDN)
	if err != nil {
		return nil, nil // Entry not found, return nil without error
	}

	return storageEntry, nil
}

// hasStorageObjectClass checks if the storage entry has an objectClass attribute with at least one value.
func hasStorageObjectClass(entry *storage.Entry) bool {
	if entry == nil || entry.Attributes == nil {
		return false
	}

	// Check for objectClass attribute (case-insensitive)
	for name, values := range entry.Attributes {
		if strings.EqualFold(name, "objectclass") {
			return len(values) > 0
		}
	}

	return false
}
