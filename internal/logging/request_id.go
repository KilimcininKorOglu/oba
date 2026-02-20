// Package logging provides structured logging for the Oba LDAP server.
package logging

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

// requestIDCounter is used for generating sequential request IDs.
var requestIDCounter uint64

// GenerateRequestID generates a unique request ID.
// The format is: timestamp-counter-random (e.g., "1708425600-1-a1b2c3d4")
func GenerateRequestID() string {
	// Get timestamp in seconds
	ts := time.Now().Unix()

	// Increment counter
	counter := atomic.AddUint64(&requestIDCounter, 1)

	// Generate random suffix
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to counter-only if random fails
		return formatRequestID(ts, counter, "0000")
	}

	return formatRequestID(ts, counter, hex.EncodeToString(randomBytes))
}

// formatRequestID formats the request ID components.
func formatRequestID(ts int64, counter uint64, random string) string {
	// Use a simple format: hex timestamp + counter + random
	return hex.EncodeToString([]byte{
		byte(ts >> 24), byte(ts >> 16), byte(ts >> 8), byte(ts),
	}) + "-" + formatCounter(counter) + "-" + random
}

// formatCounter formats the counter as a hex string.
func formatCounter(counter uint64) string {
	return hex.EncodeToString([]byte{
		byte(counter >> 8), byte(counter),
	})
}
