// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"strings"

	"github.com/oba-ldap/oba/internal/server"
)

// ModifyEntry modifies an existing entry with proper validation.
// This method provides additional validation and error handling beyond the basic Modify method.
// Returns ErrEntryNotFound if the entry does not exist.
// Returns ErrInvalidEntry if the modifications would result in an invalid entry.
func (b *ObaBackend) ModifyEntry(dn string, changes []server.Modification) error {
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

	// Check if entry exists
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
		case server.ModifyAdd:
			// Add values to attribute
			for _, value := range mod.Values {
				entry.AddAttributeValue(attrName, value)
			}

		case server.ModifyDelete:
			if len(mod.Values) == 0 {
				// Delete entire attribute
				entry.DeleteAttribute(attrName)
			} else {
				// Delete specific values
				for _, value := range mod.Values {
					entry.DeleteAttributeValue(attrName, value)
				}
			}

		case server.ModifyReplace:
			if len(mod.Values) == 0 {
				// Replace with empty = delete
				entry.DeleteAttribute(attrName)
			} else {
				entry.SetAttribute(attrName, mod.Values...)
			}
		}
	}

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

	return nil
}
