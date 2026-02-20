// Package storage provides the core storage engine components for ObaDB.
package storage

// Scope represents the LDAP search scope.
type Scope int

// Scope constants.
const (
	// ScopeBase returns only the base entry itself.
	ScopeBase Scope = iota
	// ScopeOneLevel returns only the immediate children of the base entry.
	ScopeOneLevel
	// ScopeSubtree returns the base entry and all its descendants.
	ScopeSubtree
)

// IndexType represents the type of index for attribute searching.
type IndexType int

// Index type constants.
const (
	// IndexEquality supports equality searches like (uid=alice).
	IndexEquality IndexType = iota
	// IndexPresence supports presence searches like (mail=*).
	IndexPresence
	// IndexSubstring supports substring searches like (cn=*admin*).
	IndexSubstring
)

// Entry represents an LDAP entry stored in the database.
type Entry struct {
	// DN is the distinguished name of the entry.
	DN string

	// Attributes contains the entry's attribute values.
	// Key is the attribute name (lowercase), value is a slice of attribute values.
	Attributes map[string][][]byte
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}
}

// GetAttribute returns the values for the given attribute name.
func (e *Entry) GetAttribute(name string) [][]byte {
	if e.Attributes == nil {
		return nil
	}
	return e.Attributes[name]
}

// HasAttribute returns true if the entry has the given attribute.
func (e *Entry) HasAttribute(name string) bool {
	if e.Attributes == nil {
		return false
	}
	_, ok := e.Attributes[name]
	return ok
}

// SetAttribute sets the values for the given attribute name.
func (e *Entry) SetAttribute(name string, values [][]byte) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][][]byte)
	}
	e.Attributes[name] = values
}

// AddAttributeValue adds a value to the given attribute.
func (e *Entry) AddAttributeValue(name string, value []byte) {
	if e.Attributes == nil {
		e.Attributes = make(map[string][][]byte)
	}
	e.Attributes[name] = append(e.Attributes[name], value)
}

// SetStringAttribute sets string values for the given attribute name.
func (e *Entry) SetStringAttribute(name string, values ...string) {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		byteValues[i] = []byte(v)
	}
	e.SetAttribute(name, byteValues)
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	if e == nil {
		return nil
	}

	clone := &Entry{
		DN:         e.DN,
		Attributes: make(map[string][][]byte, len(e.Attributes)),
	}

	for k, v := range e.Attributes {
		values := make([][]byte, len(v))
		for i, val := range v {
			values[i] = make([]byte, len(val))
			copy(values[i], val)
		}
		clone.Attributes[k] = values
	}

	return clone
}

// FilterMatcher is an interface for filter evaluation.
// This allows the filter package to implement matching without creating
// an import cycle.
type FilterMatcher interface {
	// Match returns true if the entry matches the filter.
	Match(entry *Entry) bool
}

// Iterator provides iteration over search results.
type Iterator interface {
	// Next advances to the next entry and returns true if successful.
	Next() bool

	// Entry returns the current entry.
	Entry() *Entry

	// Error returns any error encountered during iteration.
	Error() error

	// Close releases resources held by the iterator.
	Close()
}

// EngineStats contains statistics about the storage engine.
type EngineStats struct {
	// TotalPages is the total number of pages in the database.
	TotalPages uint64

	// FreePages is the number of free pages available.
	FreePages uint64

	// UsedPages is the number of pages in use.
	UsedPages uint64

	// EntryCount is the total number of entries in the database.
	EntryCount uint64

	// IndexCount is the number of indexes.
	IndexCount int

	// ActiveTransactions is the number of active transactions.
	ActiveTransactions int

	// BufferPoolSize is the number of pages in the buffer pool.
	BufferPoolSize int

	// DirtyPages is the number of dirty pages in the buffer pool.
	DirtyPages int

	// WALSize is the current WAL size in bytes.
	WALSize uint64

	// LastCheckpointLSN is the LSN of the last checkpoint.
	LastCheckpointLSN uint64
}

// StorageEngine defines the interface for the ObaDB storage engine.
// It provides transaction management, entry operations, search operations,
// index management, and maintenance operations.
type StorageEngine interface {
	// Transaction management

	// Begin starts a new transaction and returns it.
	// Returns a *tx.Transaction from the tx package.
	Begin() (interface{}, error)

	// Commit commits the transaction, making all changes durable.
	// The tx parameter should be a *tx.Transaction.
	Commit(tx interface{}) error

	// Rollback aborts the transaction and undoes all changes.
	// The tx parameter should be a *tx.Transaction.
	Rollback(tx interface{}) error

	// Entry operations

	// Get retrieves an entry by its DN within the given transaction.
	// The tx parameter should be a *tx.Transaction.
	Get(tx interface{}, dn string) (*Entry, error)

	// Put stores an entry within the given transaction.
	// The tx parameter should be a *tx.Transaction.
	Put(tx interface{}, entry *Entry) error

	// Delete removes an entry by its DN within the given transaction.
	// The tx parameter should be a *tx.Transaction.
	Delete(tx interface{}, dn string) error

	// HasChildren returns true if the entry at the given DN has child entries.
	// The tx parameter should be a *tx.Transaction.
	HasChildren(tx interface{}, dn string) (bool, error)

	// Search operations

	// SearchByDN searches for entries by DN with the given scope.
	// The tx parameter should be a *tx.Transaction.
	SearchByDN(tx interface{}, baseDN string, scope Scope) Iterator

	// SearchByFilter searches for entries matching the given filter.
	// The tx parameter should be a *tx.Transaction.
	// The filter parameter should be a *filter.Filter from the filter package.
	SearchByFilter(tx interface{}, baseDN string, f interface{}) Iterator

	// Index management

	// CreateIndex creates a new index for the given attribute.
	CreateIndex(attribute string, indexType IndexType) error

	// DropIndex removes an index for the given attribute.
	DropIndex(attribute string) error

	// Maintenance

	// Checkpoint performs a checkpoint operation.
	Checkpoint() error

	// Compact compacts the database to reclaim space.
	Compact() error

	// Stats returns statistics about the storage engine.
	Stats() *EngineStats

	// Close closes the storage engine and releases all resources.
	Close() error
}
