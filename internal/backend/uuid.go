// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
package backend

import (
	"crypto/rand"
	"fmt"
)

// GenerateUUID generates a UUID v4 using crypto/rand.
// The UUID is formatted as a standard UUID string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func GenerateUUID() string {
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		// Fallback to a zero UUID if random generation fails
		return "00000000-0000-0000-0000-000000000000"
	}

	// Set version 4 (random UUID)
	uuid[6] = (uuid[6] & 0x0f) | 0x40

	// Set variant (RFC 4122)
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16])
}
