// Package backup provides LDIF export and import functionality for ObaDB.
package backup

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
)

// TestNeedsBase64Encoding tests the base64 encoding detection.
func TestNeedsBase64Encoding(t *testing.T) {
	tests := []struct {
		name     string
		value    []byte
		expected bool
	}{
		{
			name:     "empty value",
			value:    []byte{},
			expected: false,
		},
		{
			name:     "simple ASCII",
			value:    []byte("hello world"),
			expected: false,
		},
		{
			name:     "starts with space",
			value:    []byte(" hello"),
			expected: true,
		},
		{
			name:     "starts with colon",
			value:    []byte(":hello"),
			expected: true,
		},
		{
			name:     "starts with less-than",
			value:    []byte("<hello"),
			expected: true,
		},
		{
			name:     "contains newline",
			value:    []byte("hello\nworld"),
			expected: true,
		},
		{
			name:     "contains carriage return",
			value:    []byte("hello\rworld"),
			expected: true,
		},
		{
			name:     "contains NUL",
			value:    []byte("hello\x00world"),
			expected: true,
		},
		{
			name:     "contains non-ASCII",
			value:    []byte("héllo"),
			expected: true,
		},
		{
			name:     "binary data",
			value:    []byte{0x00, 0x01, 0x02, 0xFF},
			expected: true,
		},
		{
			name:     "all printable ASCII",
			value:    []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsBase64Encoding(tt.value)
			if result != tt.expected {
				t.Errorf("needsBase64Encoding(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestParseLDIF tests parsing LDIF content.
func TestParseLDIF(t *testing.T) {
	ldif := `dn: uid=alice,ou=users,dc=example,dc=com
objectclass: person
objectclass: inetOrgPerson
cn: Alice Smith
uid: alice
mail: alice@example.com

dn: uid=bob,ou=users,dc=example,dc=com
objectclass: person
cn: Bob Jones
uid: bob

`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].DN != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("First entry DN = %q, want %q", entries[0].DN, "uid=alice,ou=users,dc=example,dc=com")
	}

	cn := entries[0].GetAttribute("cn")
	if len(cn) != 1 || string(cn[0]) != "Alice Smith" {
		t.Errorf("First entry cn = %v, want [Alice Smith]", cn)
	}

	objectClass := entries[0].GetAttribute("objectclass")
	if len(objectClass) != 2 {
		t.Errorf("First entry objectclass count = %d, want 2", len(objectClass))
	}

	// Check second entry
	if entries[1].DN != "uid=bob,ou=users,dc=example,dc=com" {
		t.Errorf("Second entry DN = %q, want %q", entries[1].DN, "uid=bob,ou=users,dc=example,dc=com")
	}
}

// TestParseLDIFWithBase64 tests parsing LDIF with base64 encoded values.
func TestParseLDIFWithBase64(t *testing.T) {
	// Create base64 encoded values
	binaryValue := []byte{0x00, 0x01, 0x02, 0xFF}
	encodedValue := base64.StdEncoding.EncodeToString(binaryValue)

	ldif := `dn: uid=test,dc=example,dc=com
cn: Test User
userCertificate:: ` + encodedValue + `

`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	cert := entries[0].GetAttribute("usercertificate")
	if len(cert) != 1 {
		t.Fatalf("Expected 1 certificate value, got %d", len(cert))
	}

	if !bytes.Equal(cert[0], binaryValue) {
		t.Errorf("Certificate value = %v, want %v", cert[0], binaryValue)
	}
}

// TestParseLDIFWithBase64DN tests parsing LDIF with base64 encoded DN.
func TestParseLDIFWithBase64DN(t *testing.T) {
	// DN with special characters that needs base64 encoding
	dn := "cn=Test User,ou=users,dc=example,dc=com"
	encodedDN := base64.StdEncoding.EncodeToString([]byte(dn))

	ldif := `dn:: ` + encodedDN + `
cn: Test User

`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].DN != dn {
		t.Errorf("Entry DN = %q, want %q", entries[0].DN, dn)
	}
}

// TestParseLDIFWithLineContinuation tests parsing LDIF with line continuation.
func TestParseLDIFWithLineContinuation(t *testing.T) {
	ldif := `dn: uid=test,dc=example,dc=com
description: This is a very long description that spans
 multiple lines using the standard LDIF line
 continuation format

`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	desc := entries[0].GetAttribute("description")
	if len(desc) != 1 {
		t.Fatalf("Expected 1 description value, got %d", len(desc))
	}

	// Line continuation removes the leading space but joins lines directly
	expected := "This is a very long description that spansmultiple lines using the standard LDIF linecontinuation format"
	if string(desc[0]) != expected {
		t.Errorf("Description = %q, want %q", string(desc[0]), expected)
	}
}

// TestParseLDIFWithComments tests parsing LDIF with comments.
func TestParseLDIFWithComments(t *testing.T) {
	ldif := `# This is a comment
dn: uid=test,dc=example,dc=com
# Another comment
cn: Test User

`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].DN != "uid=test,dc=example,dc=com" {
		t.Errorf("Entry DN = %q, want %q", entries[0].DN, "uid=test,dc=example,dc=com")
	}
}

// TestParseLDIFNoTrailingNewline tests parsing LDIF without trailing newline.
func TestParseLDIFNoTrailingNewline(t *testing.T) {
	ldif := `dn: uid=test,dc=example,dc=com
cn: Test User`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
}

// TestParseLDIFMissingDN tests parsing LDIF with missing DN.
func TestParseLDIFMissingDN(t *testing.T) {
	// Entry that starts with DN but has empty DN value
	ldif := `dn:
cn: Test User
uid: test

`

	_, err := ParseLDIF(strings.NewReader(ldif))
	if err == nil {
		t.Error("Expected error for missing DN, got nil")
	}
}

// TestParseLDIFInvalidBase64 tests parsing LDIF with invalid base64.
func TestParseLDIFInvalidBase64(t *testing.T) {
	ldif := `dn: uid=test,dc=example,dc=com
cn:: not-valid-base64!!!

`

	_, err := ParseLDIF(strings.NewReader(ldif))
	if err == nil {
		t.Error("Expected error for invalid base64, got nil")
	}
}

// TestWriteLDIF tests writing entries to LDIF format.
func TestWriteLDIF(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=alice,ou=users,dc=example,dc=com", "person", "Alice Smith"),
		createTestEntry("uid=bob,ou=users,dc=example,dc=com", "person", "Bob Jones"),
	}

	var buf bytes.Buffer
	if err := WriteLDIF(&buf, entries); err != nil {
		t.Fatalf("WriteLDIF failed: %v", err)
	}

	output := buf.String()

	// Verify DN lines are present
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") {
		t.Error("Output missing Alice's DN")
	}
	if !strings.Contains(output, "dn: uid=bob,ou=users,dc=example,dc=com") {
		t.Error("Output missing Bob's DN")
	}

	// Verify attributes are present
	if !strings.Contains(output, "cn: Alice Smith") {
		t.Error("Output missing Alice's cn")
	}
	if !strings.Contains(output, "cn: Bob Jones") {
		t.Error("Output missing Bob's cn")
	}
}

// TestWriteLDIFWithBinaryData tests writing entries with binary data.
func TestWriteLDIFWithBinaryData(t *testing.T) {
	entry := storage.NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("cn", "Test User")
	entry.SetAttribute("usercertificate", [][]byte{{0x00, 0x01, 0x02, 0xFF}})

	var buf bytes.Buffer
	if err := WriteLDIF(&buf, []*storage.Entry{entry}); err != nil {
		t.Fatalf("WriteLDIF failed: %v", err)
	}

	output := buf.String()

	// Verify binary data is base64 encoded
	if !strings.Contains(output, "usercertificate::") {
		t.Error("Binary attribute should use base64 encoding (::)")
	}
}

// TestRoundTrip tests that export and import produce equivalent entries.
func TestRoundTrip(t *testing.T) {
	// Create original entries
	original := []*storage.Entry{
		createTestEntry("dc=example,dc=com", "organization", "Example Inc"),
		createTestEntry("ou=users,dc=example,dc=com", "organizationalUnit", "Users"),
		createTestEntry("uid=alice,ou=users,dc=example,dc=com", "person", "Alice Smith"),
	}

	// Add some extra attributes
	original[2].SetStringAttribute("mail", "alice@example.com")
	original[2].SetStringAttribute("uid", "alice")

	// Export to LDIF
	var buf bytes.Buffer
	if err := WriteLDIF(&buf, original); err != nil {
		t.Fatalf("WriteLDIF failed: %v", err)
	}

	// Parse back
	parsed, err := ParseLDIF(&buf)
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	// Verify count
	if len(parsed) != len(original) {
		t.Fatalf("Entry count mismatch: got %d, want %d", len(parsed), len(original))
	}

	// Verify each entry
	for i, orig := range original {
		// DN comparison (case-insensitive for LDIF)
		if strings.ToLower(parsed[i].DN) != strings.ToLower(orig.DN) {
			t.Errorf("Entry %d DN mismatch: got %q, want %q", i, parsed[i].DN, orig.DN)
		}

		// Verify attributes exist
		for attr, values := range orig.Attributes {
			parsedValues := parsed[i].GetAttribute(attr)
			if len(parsedValues) != len(values) {
				t.Errorf("Entry %d attribute %s value count mismatch: got %d, want %d",
					i, attr, len(parsedValues), len(values))
			}
		}
	}
}

// TestLDIFExporterWithEngine tests the LDIFExporter with a real storage engine.
func TestLDIFExporterWithEngine(t *testing.T) {
	dir := t.TempDir()

	db, err := engine.Open(dir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Insert test entries
	entries := []*storage.Entry{
		createTestEntry("dc=example,dc=com", "organization", "Example Inc"),
		createTestEntry("ou=users,dc=example,dc=com", "organizationalUnit", "Users"),
		createTestEntry("uid=alice,ou=users,dc=example,dc=com", "person", "Alice Smith"),
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	for _, entry := range entries {
		if err := db.Put(tx, entry); err != nil {
			t.Fatalf("Failed to put entry: %v", err)
		}
	}

	if err := db.Commit(tx); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Export entries
	exporter := NewLDIFExporter(db)
	var buf bytes.Buffer
	if err := exporter.Export(&buf, "dc=example,dc=com"); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	output := buf.String()

	// Verify output contains expected entries
	if !strings.Contains(output, "dc=example,dc=com") {
		t.Error("Export missing base entry")
	}
	if !strings.Contains(output, "ou=users,dc=example,dc=com") {
		t.Error("Export missing users OU")
	}
	if !strings.Contains(output, "uid=alice,ou=users,dc=example,dc=com") {
		t.Error("Export missing alice entry")
	}
}

// TestLDIFImporterWithEngine tests the LDIFImporter with a real storage engine.
func TestLDIFImporterWithEngine(t *testing.T) {
	dir := t.TempDir()

	db, err := engine.Open(dir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ldif := `dn: dc=example,dc=com
objectclass: organization
cn: Example Inc

dn: ou=users,dc=example,dc=com
objectclass: organizationalUnit
cn: Users

dn: uid=alice,ou=users,dc=example,dc=com
objectclass: person
cn: Alice Smith
uid: alice
mail: alice@example.com

`

	// Import entries
	importer := NewLDIFImporter(db)
	if err := importer.Import(strings.NewReader(ldif)); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify entries were imported
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer db.Commit(tx)

	// Check alice entry
	entry, err := db.Get(tx, "uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("Failed to get alice entry: %v", err)
	}

	cn := entry.GetAttribute("cn")
	if len(cn) != 1 || string(cn[0]) != "Alice Smith" {
		t.Errorf("Alice cn = %v, want [Alice Smith]", cn)
	}

	mail := entry.GetAttribute("mail")
	if len(mail) != 1 || string(mail[0]) != "alice@example.com" {
		t.Errorf("Alice mail = %v, want [alice@example.com]", mail)
	}
}

// TestLDIFImporterBatch tests batch import.
func TestLDIFImporterBatch(t *testing.T) {
	dir := t.TempDir()

	db, err := engine.Open(dir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ldif := `dn: dc=example,dc=com
objectclass: organization
cn: Example Inc

dn: ou=users,dc=example,dc=com
objectclass: organizationalUnit
cn: Users

`

	// Import entries in batch
	importer := NewLDIFImporter(db)
	if err := importer.ImportBatch(strings.NewReader(ldif)); err != nil {
		t.Fatalf("ImportBatch failed: %v", err)
	}

	// Verify entries were imported
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer db.Commit(tx)

	iter := db.SearchByDN(tx, "dc=example,dc=com", storage.ScopeSubtree)
	count := 0
	for iter.Next() {
		count++
	}
	iter.Close()

	if count != 2 {
		t.Errorf("Expected 2 entries, got %d", count)
	}
}

// TestLDIFExporterNilEngine tests exporter with nil engine.
func TestLDIFExporterNilEngine(t *testing.T) {
	exporter := NewLDIFExporter(nil)
	var buf bytes.Buffer
	err := exporter.Export(&buf, "dc=example,dc=com")
	if err != ErrNilEngine {
		t.Errorf("Expected ErrNilEngine, got %v", err)
	}
}

// TestLDIFImporterNilEngine tests importer with nil engine.
func TestLDIFImporterNilEngine(t *testing.T) {
	importer := NewLDIFImporter(nil)
	err := importer.Import(strings.NewReader("dn: dc=test\ncn: test\n\n"))
	if err != ErrNilEngine {
		t.Errorf("Expected ErrNilEngine, got %v", err)
	}
}

// TestLDIFExporterNilWriter tests exporter with nil writer.
func TestLDIFExporterNilWriter(t *testing.T) {
	dir := t.TempDir()
	db, err := engine.Open(dir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	exporter := NewLDIFExporter(db)
	err = exporter.Export(nil, "dc=example,dc=com")
	if err != ErrExportFailed {
		t.Errorf("Expected ErrExportFailed, got %v", err)
	}
}

// TestLDIFImporterNilReader tests importer with nil reader.
func TestLDIFImporterNilReader(t *testing.T) {
	dir := t.TempDir()
	db, err := engine.Open(dir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	importer := NewLDIFImporter(db)
	err = importer.Import(nil)
	if err != ErrEmptyReader {
		t.Errorf("Expected ErrEmptyReader, got %v", err)
	}
}

// TestLDIFCompatibleWithLdapsearch tests that output is compatible with ldapsearch format.
func TestLDIFCompatibleWithLdapsearch(t *testing.T) {
	// Create an entry similar to ldapsearch output
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "top", "person", "organizationalPerson", "inetOrgPerson")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("sn", "Smith")
	entry.SetStringAttribute("givenname", "Alice")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com")

	var buf bytes.Buffer
	if err := WriteLDIF(&buf, []*storage.Entry{entry}); err != nil {
		t.Fatalf("WriteLDIF failed: %v", err)
	}

	output := buf.String()

	// Verify format matches ldapsearch output style
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatal("Output too short")
	}

	// First line should be DN
	if !strings.HasPrefix(lines[0], "dn:") {
		t.Errorf("First line should start with 'dn:', got %q", lines[0])
	}

	// Verify attribute format (name: value or name:: base64value)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":") {
			t.Errorf("Invalid attribute line format: %q", line)
		}
	}
}

// TestExportImportRoundTripWithEngine tests full round-trip with engine.
func TestExportImportRoundTripWithEngine(t *testing.T) {
	// Create source database
	srcDir := t.TempDir()
	srcDB, err := engine.Open(srcDir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}

	// Insert test entries
	entries := []*storage.Entry{
		createTestEntry("dc=example,dc=com", "organization", "Example Inc"),
		createTestEntry("ou=users,dc=example,dc=com", "organizationalUnit", "Users"),
		createTestEntry("uid=alice,ou=users,dc=example,dc=com", "person", "Alice Smith"),
		createTestEntry("uid=bob,ou=users,dc=example,dc=com", "person", "Bob Jones"),
	}

	// Add extra attributes
	entries[2].SetStringAttribute("mail", "alice@example.com")
	entries[2].SetStringAttribute("uid", "alice")
	entries[3].SetStringAttribute("mail", "bob@example.com")
	entries[3].SetStringAttribute("uid", "bob")

	tx, err := srcDB.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	for _, entry := range entries {
		if err := srcDB.Put(tx, entry); err != nil {
			t.Fatalf("Failed to put entry: %v", err)
		}
	}

	if err := srcDB.Commit(tx); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Export from source
	exporter := NewLDIFExporter(srcDB)
	var buf bytes.Buffer
	if err := exporter.Export(&buf, "dc=example,dc=com"); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	srcDB.Close()

	// Create destination database
	dstDir := t.TempDir()
	dstDB, err := engine.Open(dstDir, storage.DefaultEngineOptions())
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dstDB.Close()

	// Import to destination
	importer := NewLDIFImporter(dstDB)
	if err := importer.Import(&buf); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify all entries were imported
	tx, err = dstDB.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer dstDB.Commit(tx)

	iter := dstDB.SearchByDN(tx, "dc=example,dc=com", storage.ScopeSubtree)
	count := 0
	for iter.Next() {
		count++
	}
	iter.Close()

	if count != 4 {
		t.Errorf("Expected 4 entries after import, got %d", count)
	}

	// Verify specific entry
	alice, err := dstDB.Get(tx, "uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("Failed to get alice: %v", err)
	}

	mail := alice.GetAttribute("mail")
	if len(mail) != 1 || string(mail[0]) != "alice@example.com" {
		t.Errorf("Alice mail = %v, want [alice@example.com]", mail)
	}
}

// TestParseLDIFEmpty tests parsing empty LDIF.
func TestParseLDIFEmpty(t *testing.T) {
	entries, err := ParseLDIF(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

// TestParseLDIFOnlyComments tests parsing LDIF with only comments.
func TestParseLDIFOnlyComments(t *testing.T) {
	ldif := `# This is a comment
# Another comment
`

	entries, err := ParseLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("ParseLDIF failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

// Helper function to create test entries.
func createTestEntry(dn, objectClass, cn string) *storage.Entry {
	entry := storage.NewEntry(dn)
	entry.SetStringAttribute("objectclass", objectClass)
	entry.SetStringAttribute("cn", cn)
	return entry
}
