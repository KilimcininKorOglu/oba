// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"errors"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/radix"
)

// ModifyDN errors.
var (
	// ErrNewSuperiorNotFound is returned when the new superior DN does not exist.
	ErrNewSuperiorNotFound = errors.New("backend: new superior not found")
	// ErrAffectsMultipleDSAs is returned when the operation would affect multiple DSAs.
	ErrAffectsMultipleDSAs = errors.New("backend: operation affects multiple DSAs")
)

// ModifyDNRequest represents a request to rename or move an entry.
type ModifyDNRequest struct {
	// DN is the distinguished name of the entry to rename/move.
	DN string
	// NewRDN is the new relative distinguished name.
	NewRDN string
	// DeleteOldRDN indicates whether to delete the old RDN attribute values.
	DeleteOldRDN bool
	// NewSuperior is the optional new parent DN (for moving entries).
	NewSuperior string
}

// ModifyDN renames or moves an entry in the directory.
// This operation:
// 1. Validates the entry exists
// 2. Constructs the new DN from NewRDN and NewSuperior
// 3. Checks if the new DN already exists
// 4. Checks if the new parent exists (if NewSuperior specified)
// 5. Updates the entry DN in storage
// 6. If DeleteOldRDN is true, removes the old RDN attribute values
// 7. Updates any children entries if moving a subtree
func (b *ObaBackend) ModifyDN(req *ModifyDNRequest) error {
	if req == nil {
		return ErrInvalidEntry
	}

	if req.DN == "" {
		return ErrInvalidDN
	}

	if req.NewRDN == "" {
		return ErrInvalidDN
	}

	normalizedDN := normalizeDN(req.DN)
	normalizedNewRDN := normalizeDN(req.NewRDN)

	// Start a read transaction to validate
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

	// Calculate the new DN
	newDN, err := b.calculateNewDN(normalizedDN, normalizedNewRDN, req.NewSuperior)
	if err != nil {
		b.engine.Rollback(txn)
		return err
	}

	// Check if new DN already exists (unless it's the same as the old DN)
	if !strings.EqualFold(normalizedDN, newDN) {
		_, err = b.engine.Get(txn, newDN)
		if err == nil {
			b.engine.Rollback(txn)
			return ErrEntryExists
		}
	}

	// If NewSuperior is specified, verify it exists
	if req.NewSuperior != "" {
		normalizedNewSuperior := normalizeDN(req.NewSuperior)
		_, err = b.engine.Get(txn, normalizedNewSuperior)
		if err != nil {
			b.engine.Rollback(txn)
			return ErrNewSuperiorNotFound
		}
	}

	// Check if entry has children (for subtree move) - not supported in cluster mode yet
	hasChildren, err := b.hasChildren(txn, normalizedDN)
	if err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Convert to backend entry for modification
	entry := convertFromStorageEntry(storageEntry)

	// Handle old RDN attribute deletion if requested
	if req.DeleteOldRDN {
		oldRDN, err := radix.GetRDN(normalizedDN)
		if err == nil {
			b.removeRDNAttribute(entry, oldRDN)
		}
	}

	// Add new RDN attribute values
	b.addRDNAttribute(entry, normalizedNewRDN)

	// Update the entry's DN
	entry.DN = newDN

	// Enforce OU placement rules for user/group object classes.
	if err := validateEntryPlacement(entry); err != nil {
		return err
	}

	// Convert back to storage entry
	modifiedStorageEntry := convertToStorageEntry(entry)

	// Close read transaction before cluster write
	b.engine.Rollback(txn)

	// If cluster writer is set, route through Raft consensus (atomic operation)
	if b.clusterWriter != nil {
		// Note: subtree moves not supported in cluster mode yet
		if hasChildren {
			return errors.New("subtree moves not supported in cluster mode")
		}
		if err := b.clusterWriter.ModifyDN(normalizedDN, modifiedStorageEntry); err != nil {
			return wrapStorageError(err)
		}
		return nil
	}

	// Standalone mode: direct write with transaction
	txn, err = b.engine.Begin()
	if err != nil {
		return wrapStorageError(err)
	}

	// Delete the old entry
	if err := b.engine.Delete(txn, normalizedDN); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// Put with new DN
	if err := b.engine.Put(txn, modifiedStorageEntry); err != nil {
		b.engine.Rollback(txn)
		return wrapStorageError(err)
	}

	// If entry has children, update their DNs as well
	if hasChildren {
		if err := b.updateChildrenDNs(txn, normalizedDN, newDN); err != nil {
			b.engine.Rollback(txn)
			return err
		}
	}

	// Commit the transaction
	if err := b.engine.Commit(txn); err != nil {
		return wrapStorageError(err)
	}

	return nil
}

// calculateNewDN calculates the new DN based on the new RDN and optional new superior.
func (b *ObaBackend) calculateNewDN(oldDN, newRDN, newSuperior string) (string, error) {
	var parentDN string

	if newSuperior != "" {
		// Use the new superior as the parent
		parentDN = normalizeDN(newSuperior)
	} else {
		// Get the parent of the old DN
		parent, err := radix.GetParentDN(oldDN)
		if err != nil {
			return "", ErrInvalidDN
		}
		parentDN = parent
	}

	// Construct the new DN
	if parentDN == "" {
		// Entry is at root level
		return newRDN, nil
	}

	return newRDN + "," + parentDN, nil
}

// hasChildren checks if an entry has any children.
func (b *ObaBackend) hasChildren(txn interface{}, dn string) (bool, error) {
	// Search for immediate children using one-level scope
	iter := b.engine.SearchByDN(txn, dn, storage.ScopeOneLevel)
	defer iter.Close()

	// If there's at least one child, return true
	if iter.Next() {
		return true, nil
	}

	return false, iter.Error()
}

// updateChildrenDNs updates the DNs of all children when moving a subtree.
func (b *ObaBackend) updateChildrenDNs(txn interface{}, oldParentDN, newParentDN string) error {
	// Get all descendants using subtree scope
	iter := b.engine.SearchByDN(txn, oldParentDN, storage.ScopeSubtree)
	defer iter.Close()

	var children []*storage.Entry
	for iter.Next() {
		entry := iter.Entry()
		if entry == nil {
			continue
		}
		// Skip the parent entry itself (already handled)
		if strings.EqualFold(entry.DN, oldParentDN) {
			continue
		}
		children = append(children, entry.Clone())
	}

	if err := iter.Error(); err != nil {
		return wrapStorageError(err)
	}

	// Update each child's DN
	for _, child := range children {
		oldChildDN := child.DN

		// Calculate new child DN by replacing the old parent prefix with new parent
		newChildDN := b.replaceParentDN(oldChildDN, oldParentDN, newParentDN)

		// Delete old entry
		if err := b.engine.Delete(txn, oldChildDN); err != nil {
			return wrapStorageError(err)
		}

		// Update DN and put new entry
		child.DN = newChildDN
		if err := b.engine.Put(txn, child); err != nil {
			return wrapStorageError(err)
		}
	}

	return nil
}

// replaceParentDN replaces the parent portion of a child DN with a new parent DN.
func (b *ObaBackend) replaceParentDN(childDN, oldParentDN, newParentDN string) string {
	// Get the relative part of the child DN (the part before the old parent)
	childLower := strings.ToLower(childDN)
	oldParentLower := strings.ToLower(oldParentDN)

	// Find where the old parent starts in the child DN
	idx := strings.Index(childLower, oldParentLower)
	if idx == -1 {
		return childDN // Should not happen, but return unchanged
	}

	// Get the relative part (everything before the old parent)
	relativePart := childDN[:idx]

	// Remove trailing comma if present
	relativePart = strings.TrimSuffix(relativePart, ",")

	// Construct new DN
	if relativePart == "" {
		return newParentDN
	}

	return relativePart + "," + newParentDN
}

// removeRDNAttribute removes the attribute values from the old RDN.
func (b *ObaBackend) removeRDNAttribute(entry *Entry, rdn string) {
	// Parse the RDN to get attribute type and value
	attrType, attrValue := parseRDNComponent(rdn)
	if attrType == "" {
		return
	}

	// Remove the specific value from the attribute
	entry.DeleteAttributeValue(attrType, attrValue)
}

// addRDNAttribute adds the attribute values from the new RDN.
func (b *ObaBackend) addRDNAttribute(entry *Entry, rdn string) {
	// Parse the RDN to get attribute type and value
	attrType, attrValue := parseRDNComponent(rdn)
	if attrType == "" {
		return
	}

	// Check if the value already exists
	existingValues := entry.GetAttribute(attrType)
	for _, v := range existingValues {
		if strings.EqualFold(v, attrValue) {
			return // Value already exists
		}
	}

	// Add the new value
	entry.AddAttributeValue(attrType, attrValue)
}

// parseRDNComponent parses an RDN component into attribute type and value.
// Example: "uid=alice" -> ("uid", "alice")
func parseRDNComponent(rdn string) (string, string) {
	idx := strings.Index(rdn, "=")
	if idx == -1 {
		return "", ""
	}

	attrType := strings.TrimSpace(rdn[:idx])
	attrValue := strings.TrimSpace(rdn[idx+1:])

	return strings.ToLower(attrType), attrValue
}
