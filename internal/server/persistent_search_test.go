package server

import (
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

func TestParsePersistentSearchControl(t *testing.T) {
	// Create a valid Persistent Search Control value
	encoder := ber.NewBEREncoder(32)
	seqPos := encoder.BeginSequence()
	encoder.WriteInteger(15) // changeTypes: all (1+2+4+8)
	encoder.WriteBoolean(true) // changesOnly
	encoder.WriteBoolean(true) // returnECs
	encoder.EndSequence(seqPos)

	ctrl := ldap.Control{
		OID:         PersistentSearchOID,
		Criticality: true,
		Value:       encoder.Bytes(),
	}

	psc, err := ParsePersistentSearchControl(ctrl)
	if err != nil {
		t.Fatalf("ParsePersistentSearchControl() error = %v", err)
	}

	if psc == nil {
		t.Fatal("ParsePersistentSearchControl() returned nil")
	}

	if psc.ChangeTypes != 15 {
		t.Errorf("ChangeTypes = %d, want 15", psc.ChangeTypes)
	}

	if !psc.ChangesOnly {
		t.Error("ChangesOnly = false, want true")
	}

	if !psc.ReturnECs {
		t.Error("ReturnECs = false, want true")
	}

	if !psc.Criticality {
		t.Error("Criticality = false, want true")
	}
}

func TestParsePersistentSearchControlWrongOID(t *testing.T) {
	ctrl := ldap.Control{
		OID:   "1.2.3.4.5",
		Value: []byte{},
	}

	psc, err := ParsePersistentSearchControl(ctrl)
	if err != nil {
		t.Fatalf("ParsePersistentSearchControl() error = %v", err)
	}

	if psc != nil {
		t.Error("ParsePersistentSearchControl() should return nil for wrong OID")
	}
}

func TestParsePersistentSearchControlEmptyValue(t *testing.T) {
	ctrl := ldap.Control{
		OID:   PersistentSearchOID,
		Value: []byte{},
	}

	psc, err := ParsePersistentSearchControl(ctrl)
	if err != nil {
		t.Fatalf("ParsePersistentSearchControl() error = %v", err)
	}

	if psc == nil {
		t.Fatal("ParsePersistentSearchControl() returned nil")
	}

	// Should have defaults
	if psc.ChangeTypes != (ChangeTypeAdd | ChangeTypeDelete | ChangeTypeModify | ChangeTypeModDN) {
		t.Errorf("ChangeTypes = %d, want %d", psc.ChangeTypes, ChangeTypeAdd|ChangeTypeDelete|ChangeTypeModify|ChangeTypeModDN)
	}
}

func TestFindPersistentSearchControl(t *testing.T) {
	encoder := ber.NewBEREncoder(32)
	seqPos := encoder.BeginSequence()
	encoder.WriteInteger(1) // changeTypes: add only
	encoder.WriteBoolean(false)
	encoder.WriteBoolean(false)
	encoder.EndSequence(seqPos)

	controls := []ldap.Control{
		{OID: "1.2.3.4.5", Value: []byte{}},
		{OID: PersistentSearchOID, Value: encoder.Bytes()},
		{OID: "5.4.3.2.1", Value: []byte{}},
	}

	psc, err := FindPersistentSearchControl(controls)
	if err != nil {
		t.Fatalf("FindPersistentSearchControl() error = %v", err)
	}

	if psc == nil {
		t.Fatal("FindPersistentSearchControl() returned nil")
	}

	if psc.ChangeTypes != 1 {
		t.Errorf("ChangeTypes = %d, want 1", psc.ChangeTypes)
	}
}

func TestFindPersistentSearchControlNotFound(t *testing.T) {
	controls := []ldap.Control{
		{OID: "1.2.3.4.5", Value: []byte{}},
	}

	psc, err := FindPersistentSearchControl(controls)
	if err != nil {
		t.Fatalf("FindPersistentSearchControl() error = %v", err)
	}

	if psc != nil {
		t.Error("FindPersistentSearchControl() should return nil when not found")
	}
}

func TestEntryChangeNotificationEncode(t *testing.T) {
	ecn := &EntryChangeNotification{
		ChangeType:   ChangeTypeAdd,
		ChangeNumber: 12345,
	}

	data, err := ecn.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Encode() returned empty data")
	}

	// Verify it's a valid SEQUENCE
	if data[0] != 0x30 {
		t.Errorf("First byte = 0x%02x, want 0x30 (SEQUENCE)", data[0])
	}
}

func TestEntryChangeNotificationEncodeWithPreviousDN(t *testing.T) {
	ecn := &EntryChangeNotification{
		ChangeType:   ChangeTypeModDN,
		PreviousDN:   "cn=old,dc=example,dc=com",
		ChangeNumber: 100,
	}

	data, err := ecn.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Encode() returned empty data")
	}
}

func TestEntryChangeNotificationToLDAPControl(t *testing.T) {
	ecn := &EntryChangeNotification{
		ChangeType:   ChangeTypeModify,
		ChangeNumber: 999,
	}

	ctrl, err := ecn.ToLDAPControl()
	if err != nil {
		t.Fatalf("ToLDAPControl() error = %v", err)
	}

	if ctrl.OID != EntryChangeNotificationOID {
		t.Errorf("OID = %q, want %q", ctrl.OID, EntryChangeNotificationOID)
	}

	if ctrl.Criticality {
		t.Error("Criticality should be false")
	}

	if len(ctrl.Value) == 0 {
		t.Error("Value should not be empty")
	}
}

func TestChangeTypeConstants(t *testing.T) {
	// Verify change type constants match the spec
	if ChangeTypeAdd != 1 {
		t.Errorf("ChangeTypeAdd = %d, want 1", ChangeTypeAdd)
	}
	if ChangeTypeDelete != 2 {
		t.Errorf("ChangeTypeDelete = %d, want 2", ChangeTypeDelete)
	}
	if ChangeTypeModify != 4 {
		t.Errorf("ChangeTypeModify = %d, want 4", ChangeTypeModify)
	}
	if ChangeTypeModDN != 8 {
		t.Errorf("ChangeTypeModDN = %d, want 8", ChangeTypeModDN)
	}
}
