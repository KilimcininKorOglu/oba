// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

// DeleteEntry removes an entry from the directory with proper validation.
// This method checks for children before deletion and returns appropriate errors.
// Returns ErrEntryNotFound if the entry does not exist.
// Returns ErrNotAllowedOnNonLeaf if the entry has children.
func (b *ObaBackend) DeleteEntry(dn string) error {
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

	// Check if entry has children (LDAP doesn't allow deleting non-leaf entries)
	hasChildren, err := b.engine.HasChildren(txn, normalizedDN)
	if err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	if hasChildren {
		b.engine.Rollback(txn)
		return ErrNotAllowedOnNonLeaf
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

	return nil
}
