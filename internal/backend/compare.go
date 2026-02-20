// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

// GetEntry retrieves an entry by DN for external use.
// Returns nil if the entry does not exist.
func (b *ObaBackend) GetEntry(dn string) (*Entry, error) {
	return b.getEntry(normalizeDN(dn))
}
