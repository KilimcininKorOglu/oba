// Package backup provides LDIF export and import functionality for ObaDB.
package backup

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// LDIF errors.
var (
	ErrInvalidLDIF     = errors.New("invalid LDIF format")
	ErrMissingDN       = errors.New("missing DN in LDIF entry")
	ErrInvalidBase64   = errors.New("invalid base64 encoding")
	ErrEmptyReader     = errors.New("empty reader")
	ErrTransactionFail = errors.New("transaction failed")
)

// LDIFExporter exports entries from ObaDB to LDIF format.
type LDIFExporter struct {
	engine storage.StorageEngine
}

// NewLDIFExporter creates a new LDIFExporter with the given storage engine.
func NewLDIFExporter(engine storage.StorageEngine) *LDIFExporter {
	return &LDIFExporter{
		engine: engine,
	}
}

// Export exports all entries under the given baseDN to the writer in LDIF format.
// It uses subtree scope to include all descendants.
func (e *LDIFExporter) Export(w io.Writer, baseDN string) error {
	if e.engine == nil {
		return ErrNilEngine
	}

	if w == nil {
		return ErrExportFailed
	}

	tx, err := e.engine.Begin()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFail, err)
	}
	defer e.engine.Rollback(tx)

	iter := e.engine.SearchByDN(tx, baseDN, storage.ScopeSubtree)
	defer iter.Close()

	entryCount := 0
	for iter.Next() {
		entry := iter.Entry()
		if entry == nil {
			continue
		}

		if err := e.writeEntry(w, entry); err != nil {
			return fmt.Errorf("%w: %v", ErrExportFailed, err)
		}
		entryCount++
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("%w: %v", ErrExportFailed, err)
	}

	return nil
}

// ExportEntry exports a single entry to the writer in LDIF format.
func (e *LDIFExporter) ExportEntry(w io.Writer, entry *storage.Entry) error {
	if w == nil || entry == nil {
		return ErrExportFailed
	}
	return e.writeEntry(w, entry)
}

// writeEntry writes a single entry in LDIF format.
func (e *LDIFExporter) writeEntry(w io.Writer, entry *storage.Entry) error {
	// Write DN
	if needsBase64Encoding([]byte(entry.DN)) {
		if _, err := fmt.Fprintf(w, "dn:: %s\n", base64.StdEncoding.EncodeToString([]byte(entry.DN))); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "dn: %s\n", entry.DN); err != nil {
			return err
		}
	}

	// Write attributes in sorted order for consistent output
	attrNames := getSortedAttributeNames(entry.Attributes)
	for _, attr := range attrNames {
		values := entry.Attributes[attr]
		for _, value := range values {
			if needsBase64Encoding(value) {
				if _, err := fmt.Fprintf(w, "%s:: %s\n", attr, base64.StdEncoding.EncodeToString(value)); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "%s: %s\n", attr, string(value)); err != nil {
					return err
				}
			}
		}
	}

	// Write empty line to separate entries
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return nil
}

// getSortedAttributeNames returns attribute names in sorted order.
func getSortedAttributeNames(attrs map[string][][]byte) []string {
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	// Simple insertion sort for small number of attributes
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}
	return names
}

// needsBase64Encoding checks if a value needs base64 encoding.
// According to RFC 2849, values need base64 encoding if they:
// - Contain non-printable characters (< 0x20 or > 0x7E, except for space)
// - Start with a space, colon, or less-than sign
// - Contain NUL characters
// - Contain line breaks
func needsBase64Encoding(value []byte) bool {
	if len(value) == 0 {
		return false
	}

	// Check first character for special cases
	first := value[0]
	if first == ' ' || first == ':' || first == '<' {
		return true
	}

	// Check all characters
	for _, b := range value {
		// NUL character
		if b == 0 {
			return true
		}
		// Line breaks
		if b == '\n' || b == '\r' {
			return true
		}
		// Non-printable characters (outside ASCII printable range)
		if b < 0x20 || b > 0x7E {
			return true
		}
	}

	return false
}

// LDIFImporter imports entries from LDIF format into ObaDB.
type LDIFImporter struct {
	engine storage.StorageEngine
}

// NewLDIFImporter creates a new LDIFImporter with the given storage engine.
func NewLDIFImporter(engine storage.StorageEngine) *LDIFImporter {
	return &LDIFImporter{
		engine: engine,
	}
}

// Import imports entries from the reader in LDIF format.
// Each entry is imported in its own transaction.
func (i *LDIFImporter) Import(r io.Reader) error {
	if i.engine == nil {
		return ErrNilEngine
	}

	if r == nil {
		return ErrEmptyReader
	}

	entries, err := i.Parse(r)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		tx, err := i.engine.Begin()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTransactionFail, err)
		}

		if err := i.engine.Put(tx, entry); err != nil {
			i.engine.Rollback(tx)
			return fmt.Errorf("%w: failed to import entry %s: %v", ErrImportFailed, entry.DN, err)
		}

		if err := i.engine.Commit(tx); err != nil {
			return fmt.Errorf("%w: failed to commit entry %s: %v", ErrImportFailed, entry.DN, err)
		}
	}

	return nil
}

// ImportBatch imports entries from the reader in a single transaction.
// This is more efficient for large imports but less safe if an error occurs.
func (i *LDIFImporter) ImportBatch(r io.Reader) error {
	if i.engine == nil {
		return ErrNilEngine
	}

	if r == nil {
		return ErrEmptyReader
	}

	entries, err := i.Parse(r)
	if err != nil {
		return err
	}

	tx, err := i.engine.Begin()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFail, err)
	}

	for _, entry := range entries {
		if err := i.engine.Put(tx, entry); err != nil {
			i.engine.Rollback(tx)
			return fmt.Errorf("%w: failed to import entry %s: %v", ErrImportFailed, entry.DN, err)
		}
	}

	if err := i.engine.Commit(tx); err != nil {
		return fmt.Errorf("%w: %v", ErrImportFailed, err)
	}

	return nil
}

// Parse parses LDIF content and returns entries without importing them.
func (i *LDIFImporter) Parse(r io.Reader) ([]*storage.Entry, error) {
	if r == nil {
		return nil, ErrEmptyReader
	}

	scanner := bufio.NewScanner(r)
	var entries []*storage.Entry
	var entry *storage.Entry
	var continuedLine string

	for scanner.Scan() {
		line := scanner.Text()

		// Handle line continuation (lines starting with single space)
		if len(line) > 0 && line[0] == ' ' {
			continuedLine += line[1:]
			continue
		}

		// Process any accumulated continued line
		if continuedLine != "" {
			if entry != nil {
				if err := processLine(entry, continuedLine); err != nil {
					return nil, err
				}
			}
			continuedLine = ""
		}

		// Skip comments
		if len(line) > 0 && line[0] == '#' {
			continue
		}

		// Empty line marks end of entry
		if line == "" {
			if entry != nil {
				if entry.DN == "" {
					return nil, ErrMissingDN
				}
				entries = append(entries, entry)
				entry = nil
			}
			continue
		}

		// Start new entry or add attribute
		if strings.HasPrefix(strings.ToLower(line), "dn:") {
			// Start new entry
			entry = storage.NewEntry("")
			if err := processDNLine(entry, line); err != nil {
				return nil, err
			}
		} else if entry != nil {
			// Add attribute to current entry
			continuedLine = line
		}
	}

	// Process any remaining continued line
	if continuedLine != "" && entry != nil {
		if err := processLine(entry, continuedLine); err != nil {
			return nil, err
		}
	}

	// Handle last entry if file doesn't end with empty line
	if entry != nil {
		if entry.DN == "" {
			return nil, ErrMissingDN
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLDIF, err)
	}

	return entries, nil
}

// processDNLine processes a DN line and sets the entry's DN.
func processDNLine(entry *storage.Entry, line string) error {
	// Check for base64 encoded DN (dn::)
	if strings.HasPrefix(line, "dn::") || strings.HasPrefix(line, "DN::") {
		encoded := strings.TrimSpace(line[4:])
		if encoded == "" {
			return ErrMissingDN
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBase64, err)
		}
		entry.DN = string(decoded)
	} else {
		// Plain DN (dn:)
		entry.DN = strings.TrimSpace(line[3:])
	}

	if entry.DN == "" {
		return ErrMissingDN
	}

	return nil
}

// processLine processes an attribute line and adds it to the entry.
func processLine(entry *storage.Entry, line string) error {
	// Find the separator (: or ::)
	colonIdx := strings.Index(line, ":")
	if colonIdx == -1 {
		return fmt.Errorf("%w: missing colon in line: %s", ErrInvalidLDIF, line)
	}

	attr := strings.ToLower(line[:colonIdx])
	rest := line[colonIdx+1:]

	var value []byte

	// Check for base64 encoding (::)
	if len(rest) > 0 && rest[0] == ':' {
		// Base64 encoded value
		encoded := strings.TrimSpace(rest[1:])
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBase64, err)
		}
		value = decoded
	} else {
		// Plain value
		value = []byte(strings.TrimSpace(rest))
	}

	// Add attribute value
	entry.AddAttributeValue(attr, value)

	return nil
}

// ParseLDIF is a convenience function to parse LDIF content without an engine.
func ParseLDIF(r io.Reader) ([]*storage.Entry, error) {
	importer := &LDIFImporter{}
	return importer.Parse(r)
}

// WriteLDIF is a convenience function to write entries to LDIF format.
func WriteLDIF(w io.Writer, entries []*storage.Entry) error {
	exporter := &LDIFExporter{}
	for _, entry := range entries {
		if err := exporter.writeEntry(w, entry); err != nil {
			return err
		}
	}
	return nil
}
