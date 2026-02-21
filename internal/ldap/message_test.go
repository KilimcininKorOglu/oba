package ldap

import (
	"bytes"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// Helper function to create a valid LDAP message with a BindRequest
func createBindRequestMessage(msgID int) []byte {
	encoder := ber.NewBEREncoder(128)

	// Start LDAPMessage SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write messageID
	encoder.WriteInteger(int64(msgID))

	// Write BindRequest [APPLICATION 0]
	// BindRequest ::= [APPLICATION 0] SEQUENCE {
	//     version        INTEGER (1 .. 127),
	//     name           LDAPDN,
	//     authentication AuthenticationChoice
	// }
	appPos := encoder.WriteApplicationTag(ApplicationBindRequest, true)

	// version = 3
	encoder.WriteInteger(3)
	// name = "" (anonymous)
	encoder.WriteOctetString([]byte(""))
	// authentication = simple [0] ""
	encoder.WriteTaggedValue(0, false, []byte(""))

	encoder.EndApplicationTag(appPos)
	encoder.EndSequence(seqPos)

	return encoder.Bytes()
}

// Helper function to create a valid LDAP message with a SearchRequest
func createSearchRequestMessage(msgID int) []byte {
	encoder := ber.NewBEREncoder(256)

	// Start LDAPMessage SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write messageID
	encoder.WriteInteger(int64(msgID))

	// Write SearchRequest [APPLICATION 3]
	appPos := encoder.WriteApplicationTag(ApplicationSearchRequest, true)

	// baseObject = "dc=example,dc=com"
	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	// scope = wholeSubtree (2)
	encoder.WriteEnumerated(2)
	// derefAliases = neverDerefAliases (0)
	encoder.WriteEnumerated(0)
	// sizeLimit = 0
	encoder.WriteInteger(0)
	// timeLimit = 0
	encoder.WriteInteger(0)
	// typesOnly = FALSE
	encoder.WriteBoolean(false)
	// filter = present "objectClass"
	encoder.WriteTaggedValue(7, false, []byte("objectClass"))
	// attributes = SEQUENCE OF (empty)
	attrSeqPos := encoder.BeginSequence()
	encoder.EndSequence(attrSeqPos)

	encoder.EndApplicationTag(appPos)
	encoder.EndSequence(seqPos)

	return encoder.Bytes()
}

// Helper function to create an UnbindRequest message
func createUnbindRequestMessage(msgID int) []byte {
	encoder := ber.NewBEREncoder(64)

	// Start LDAPMessage SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write messageID
	encoder.WriteInteger(int64(msgID))

	// Write UnbindRequest [APPLICATION 2] NULL
	// UnbindRequest is primitive (NULL) - use APPLICATION tag
	appPos := encoder.WriteApplicationTag(ApplicationUnbindRequest, false)
	// NULL has no content
	encoder.EndApplicationTag(appPos)

	encoder.EndSequence(seqPos)

	return encoder.Bytes()
}

// Helper function to create a message with controls
func createMessageWithControls(msgID int, controls []Control) []byte {
	encoder := ber.NewBEREncoder(256)

	// Start LDAPMessage SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write messageID
	encoder.WriteInteger(int64(msgID))

	// Write a simple BindRequest
	appPos := encoder.WriteApplicationTag(ApplicationBindRequest, true)
	encoder.WriteInteger(3)
	encoder.WriteOctetString([]byte(""))
	encoder.WriteTaggedValue(0, false, []byte(""))
	encoder.EndApplicationTag(appPos)

	// Write controls [0]
	if len(controls) > 0 {
		ctxPos := encoder.WriteContextTag(ContextTagControls, true)
		ctrlSeqPos := encoder.BeginSequence()

		for _, ctrl := range controls {
			ctrlPos := encoder.BeginSequence()
			encoder.WriteOctetString([]byte(ctrl.OID))
			if ctrl.Criticality {
				encoder.WriteBoolean(true)
			}
			if len(ctrl.Value) > 0 {
				encoder.WriteOctetString(ctrl.Value)
			}
			encoder.EndSequence(ctrlPos)
		}

		encoder.EndSequence(ctrlSeqPos)
		encoder.EndContextTag(ctxPos)
	}

	encoder.EndSequence(seqPos)

	return encoder.Bytes()
}

func TestParseLDAPMessage_BindRequest(t *testing.T) {
	data := createBindRequestMessage(1)

	msg, err := ParseLDAPMessage(data)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if msg.MessageID != 1 {
		t.Errorf("MessageID = %d, want 1", msg.MessageID)
	}

	if msg.Operation == nil {
		t.Fatal("Operation is nil")
	}

	if msg.Operation.Tag != ApplicationBindRequest {
		t.Errorf("Operation.Tag = %d, want %d (BindRequest)", msg.Operation.Tag, ApplicationBindRequest)
	}

	if msg.OperationType() != OperationType(ApplicationBindRequest) {
		t.Errorf("OperationType() = %v, want BindRequest", msg.OperationType())
	}

	if len(msg.Controls) != 0 {
		t.Errorf("Controls length = %d, want 0", len(msg.Controls))
	}
}

func TestParseLDAPMessage_SearchRequest(t *testing.T) {
	data := createSearchRequestMessage(42)

	msg, err := ParseLDAPMessage(data)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if msg.MessageID != 42 {
		t.Errorf("MessageID = %d, want 42", msg.MessageID)
	}

	if msg.Operation.Tag != ApplicationSearchRequest {
		t.Errorf("Operation.Tag = %d, want %d (SearchRequest)", msg.Operation.Tag, ApplicationSearchRequest)
	}
}

func TestParseLDAPMessage_UnbindRequest(t *testing.T) {
	data := createUnbindRequestMessage(3)

	msg, err := ParseLDAPMessage(data)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if msg.MessageID != 3 {
		t.Errorf("MessageID = %d, want 3", msg.MessageID)
	}

	// Note: UnbindRequest uses APPLICATION tag but is encoded differently
	// The tag number should still be identified
}

func TestParseLDAPMessage_WithControls(t *testing.T) {
	controls := []Control{
		{
			OID:         "1.2.840.113556.1.4.319",
			Criticality: true,
			Value:       []byte{0x30, 0x05, 0x02, 0x01, 0x64, 0x04, 0x00},
		},
		{
			OID:         "2.16.840.1.113730.3.4.2",
			Criticality: false,
			Value:       nil,
		},
	}

	data := createMessageWithControls(5, controls)

	msg, err := ParseLDAPMessage(data)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if msg.MessageID != 5 {
		t.Errorf("MessageID = %d, want 5", msg.MessageID)
	}

	if len(msg.Controls) != 2 {
		t.Fatalf("Controls length = %d, want 2", len(msg.Controls))
	}

	// Check first control
	if msg.Controls[0].OID != "1.2.840.113556.1.4.319" {
		t.Errorf("Controls[0].OID = %s, want 1.2.840.113556.1.4.319", msg.Controls[0].OID)
	}
	if !msg.Controls[0].Criticality {
		t.Error("Controls[0].Criticality = false, want true")
	}
	if !bytes.Equal(msg.Controls[0].Value, []byte{0x30, 0x05, 0x02, 0x01, 0x64, 0x04, 0x00}) {
		t.Errorf("Controls[0].Value mismatch")
	}

	// Check second control
	if msg.Controls[1].OID != "2.16.840.1.113730.3.4.2" {
		t.Errorf("Controls[1].OID = %s, want 2.16.840.1.113730.3.4.2", msg.Controls[1].OID)
	}
	if msg.Controls[1].Criticality {
		t.Error("Controls[1].Criticality = true, want false")
	}
}

func TestParseLDAPMessage_MessageIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		msgID   int64
		wantErr bool
	}{
		{"zero", 0, false},
		{"positive", 100, false},
		{"max valid", MaxMessageID, false},
		{"negative", -1, true},
		{"too large", MaxMessageID + 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := ber.NewBEREncoder(64)
			seqPos := encoder.BeginSequence()
			encoder.WriteInteger(tt.msgID)
			// Write a minimal operation
			appPos := encoder.WriteApplicationTag(ApplicationUnbindRequest, false)
			encoder.EndApplicationTag(appPos)
			encoder.EndSequence(seqPos)

			_, err := ParseLDAPMessage(encoder.Bytes())
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLDAPMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseLDAPMessage_EmptyData(t *testing.T) {
	_, err := ParseLDAPMessage([]byte{})
	if err != ErrEmptyMessage {
		t.Errorf("ParseLDAPMessage(empty) error = %v, want ErrEmptyMessage", err)
	}

	_, err = ParseLDAPMessage(nil)
	if err != ErrEmptyMessage {
		t.Errorf("ParseLDAPMessage(nil) error = %v, want ErrEmptyMessage", err)
	}
}

func TestParseLDAPMessage_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"not a sequence", []byte{0x02, 0x01, 0x01}},                   // INTEGER instead of SEQUENCE
		{"truncated sequence", []byte{0x30, 0x10}},                     // SEQUENCE with missing content
		{"truncated message id", []byte{0x30, 0x03, 0x02, 0x02, 0x01}}, // Truncated INTEGER
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseLDAPMessage(tt.data)
			if err == nil {
				t.Error("ParseLDAPMessage() expected error, got nil")
			}
		})
	}
}

func TestLDAPMessage_Encode(t *testing.T) {
	// Create a message
	msg := &LDAPMessage{
		MessageID: 1,
		Operation: &RawOperation{
			Tag:  ApplicationBindRequest,
			Data: []byte{0x02, 0x01, 0x03, 0x04, 0x00, 0xa0, 0x00}, // version=3, name="", auth=simple ""
		},
	}

	encoded, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseLDAPMessage(encoded)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if parsed.MessageID != msg.MessageID {
		t.Errorf("MessageID = %d, want %d", parsed.MessageID, msg.MessageID)
	}

	if parsed.Operation.Tag != msg.Operation.Tag {
		t.Errorf("Operation.Tag = %d, want %d", parsed.Operation.Tag, msg.Operation.Tag)
	}
}

func TestLDAPMessage_EncodeWithControls(t *testing.T) {
	msg := &LDAPMessage{
		MessageID: 10,
		Operation: &RawOperation{
			Tag:  ApplicationSearchRequest,
			Data: []byte{0x04, 0x00}, // Minimal search request data
		},
		Controls: []Control{
			{
				OID:         "1.2.3.4.5",
				Criticality: true,
				Value:       []byte{0x01, 0x02, 0x03},
			},
		},
	}

	encoded, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseLDAPMessage(encoded)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if len(parsed.Controls) != 1 {
		t.Fatalf("Controls length = %d, want 1", len(parsed.Controls))
	}

	if parsed.Controls[0].OID != "1.2.3.4.5" {
		t.Errorf("Controls[0].OID = %s, want 1.2.3.4.5", parsed.Controls[0].OID)
	}

	if !parsed.Controls[0].Criticality {
		t.Error("Controls[0].Criticality = false, want true")
	}

	if !bytes.Equal(parsed.Controls[0].Value, []byte{0x01, 0x02, 0x03}) {
		t.Error("Controls[0].Value mismatch")
	}
}

func TestLDAPMessage_EncodeValidation(t *testing.T) {
	// Test invalid message ID
	msg := &LDAPMessage{
		MessageID: -1,
		Operation: &RawOperation{Tag: 0, Data: []byte{}},
	}
	_, err := msg.Encode()
	if err != ErrInvalidMessageID {
		t.Errorf("Encode() with negative ID error = %v, want ErrInvalidMessageID", err)
	}

	// Test missing operation
	msg = &LDAPMessage{
		MessageID: 1,
		Operation: nil,
	}
	_, err = msg.Encode()
	if err != ErrMissingOperation {
		t.Errorf("Encode() with nil operation error = %v, want ErrMissingOperation", err)
	}
}

func TestOperationType_String(t *testing.T) {
	tests := []struct {
		op   OperationType
		want string
	}{
		{ApplicationBindRequest, "BindRequest"},
		{ApplicationBindResponse, "BindResponse"},
		{ApplicationUnbindRequest, "UnbindRequest"},
		{ApplicationSearchRequest, "SearchRequest"},
		{ApplicationSearchResultEntry, "SearchResultEntry"},
		{ApplicationSearchResultDone, "SearchResultDone"},
		{ApplicationModifyRequest, "ModifyRequest"},
		{ApplicationModifyResponse, "ModifyResponse"},
		{ApplicationAddRequest, "AddRequest"},
		{ApplicationAddResponse, "AddResponse"},
		{ApplicationDelRequest, "DelRequest"},
		{ApplicationDelResponse, "DelResponse"},
		{ApplicationModifyDNRequest, "ModifyDNRequest"},
		{ApplicationModifyDNResponse, "ModifyDNResponse"},
		{ApplicationCompareRequest, "CompareRequest"},
		{ApplicationCompareResponse, "CompareResponse"},
		{ApplicationAbandonRequest, "AbandonRequest"},
		{ApplicationSearchResultReference, "SearchResultReference"},
		{ApplicationExtendedRequest, "ExtendedRequest"},
		{ApplicationExtendedResponse, "ExtendedResponse"},
		{ApplicationIntermediateResponse, "IntermediateResponse"},
		{OperationType(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("OperationType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	// Test with underlying error
	err := NewParseError(10, "test message", ErrInvalidMessageID)
	if err.Offset != 10 {
		t.Errorf("Offset = %d, want 10", err.Offset)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %s, want 'test message'", err.Message)
	}
	if err.Unwrap() != ErrInvalidMessageID {
		t.Errorf("Unwrap() = %v, want ErrInvalidMessageID", err.Unwrap())
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() returned empty string")
	}

	// Test without underlying error
	err2 := NewParseError(5, "another message", nil)
	if err2.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", err2.Unwrap())
	}
}

func TestRoundTrip_AllOperationTypes(t *testing.T) {
	operationTypes := []int{
		ApplicationBindRequest,
		ApplicationBindResponse,
		ApplicationUnbindRequest,
		ApplicationSearchRequest,
		ApplicationSearchResultEntry,
		ApplicationSearchResultDone,
		ApplicationModifyRequest,
		ApplicationModifyResponse,
		ApplicationAddRequest,
		ApplicationAddResponse,
		ApplicationDelRequest,
		ApplicationDelResponse,
		ApplicationModifyDNRequest,
		ApplicationModifyDNResponse,
		ApplicationCompareRequest,
		ApplicationCompareResponse,
		ApplicationAbandonRequest,
		ApplicationSearchResultReference,
		ApplicationExtendedRequest,
		ApplicationExtendedResponse,
		ApplicationIntermediateResponse,
	}

	for _, opType := range operationTypes {
		t.Run(OperationType(opType).String(), func(t *testing.T) {
			msg := &LDAPMessage{
				MessageID: 100,
				Operation: &RawOperation{
					Tag:  opType,
					Data: []byte{0x04, 0x00}, // Minimal data
				},
			}

			encoded, err := msg.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			parsed, err := ParseLDAPMessage(encoded)
			if err != nil {
				t.Fatalf("ParseLDAPMessage failed: %v", err)
			}

			if parsed.Operation.Tag != opType {
				t.Errorf("Operation.Tag = %d, want %d", parsed.Operation.Tag, opType)
			}
		})
	}
}

func TestControl_DefaultCriticality(t *testing.T) {
	// Create a control with only OID (criticality should default to false)
	encoder := ber.NewBEREncoder(64)
	seqPos := encoder.BeginSequence()
	encoder.WriteInteger(1)

	appPos := encoder.WriteApplicationTag(ApplicationBindRequest, true)
	encoder.WriteInteger(3)
	encoder.WriteOctetString([]byte(""))
	encoder.WriteTaggedValue(0, false, []byte(""))
	encoder.EndApplicationTag(appPos)

	// Controls with only OID
	ctxPos := encoder.WriteContextTag(ContextTagControls, true)
	ctrlSeqPos := encoder.BeginSequence()
	ctrlPos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("1.2.3.4"))
	// No criticality, no value
	encoder.EndSequence(ctrlPos)
	encoder.EndSequence(ctrlSeqPos)
	encoder.EndContextTag(ctxPos)

	encoder.EndSequence(seqPos)

	msg, err := ParseLDAPMessage(encoder.Bytes())
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if len(msg.Controls) != 1 {
		t.Fatalf("Controls length = %d, want 1", len(msg.Controls))
	}

	if msg.Controls[0].Criticality {
		t.Error("Controls[0].Criticality = true, want false (default)")
	}
}

func TestLDAPMessage_LargeMessageID(t *testing.T) {
	// Test with maximum valid message ID
	data := createBindRequestMessage(MaxMessageID)

	msg, err := ParseLDAPMessage(data)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if msg.MessageID != MaxMessageID {
		t.Errorf("MessageID = %d, want %d", msg.MessageID, MaxMessageID)
	}

	// Encode and verify round-trip
	encoded, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	parsed, err := ParseLDAPMessage(encoded)
	if err != nil {
		t.Fatalf("ParseLDAPMessage failed: %v", err)
	}

	if parsed.MessageID != MaxMessageID {
		t.Errorf("Round-trip MessageID = %d, want %d", parsed.MessageID, MaxMessageID)
	}
}

func TestIsConstructedOperation(t *testing.T) {
	// Primitive operations
	if isConstructedOperation(ApplicationUnbindRequest) {
		t.Error("UnbindRequest should be primitive")
	}
	if isConstructedOperation(ApplicationAbandonRequest) {
		t.Error("AbandonRequest should be primitive")
	}
	if isConstructedOperation(ApplicationDelRequest) {
		t.Error("DelRequest should be primitive")
	}

	// Constructed operations
	if !isConstructedOperation(ApplicationBindRequest) {
		t.Error("BindRequest should be constructed")
	}
	if !isConstructedOperation(ApplicationSearchRequest) {
		t.Error("SearchRequest should be constructed")
	}
}
