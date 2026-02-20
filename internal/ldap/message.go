// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"github.com/oba-ldap/oba/internal/ber"
)

// ParseLDAPMessage parses a BER-encoded LDAP message envelope.
// Per RFC 4511 Section 4.1.1:
// LDAPMessage ::= SEQUENCE {
//
//	messageID       MessageID,
//	protocolOp      CHOICE { ... },
//	controls        [0] Controls OPTIONAL
//
// }
func ParseLDAPMessage(data []byte) (*LDAPMessage, error) {
	if len(data) == 0 {
		return nil, ErrEmptyMessage
	}

	decoder := ber.NewBERDecoder(data)

	// Read the outer SEQUENCE tag and length
	seqLength, err := decoder.ExpectSequence()
	if err != nil {
		return nil, NewParseError(0, "expected SEQUENCE for LDAPMessage", err)
	}

	// Calculate the end of the sequence content
	seqContentStart := decoder.Offset()
	seqContentEnd := seqContentStart + seqLength

	// Read messageID (INTEGER)
	msgID, err := decoder.ReadInteger()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read messageID", err)
	}

	// Validate message ID range per RFC 4511
	if msgID < MinMessageID || msgID > MaxMessageID {
		return nil, ErrInvalidMessageID
	}

	// Record position before reading operation tag
	opStartOffset := decoder.Offset()

	// Read protocolOp tag
	class, constructed, tagNum, err := decoder.ReadTag()
	if err != nil {
		return nil, NewParseError(opStartOffset, "failed to read protocolOp tag", err)
	}

	// Verify it's an APPLICATION tag
	if class != ber.ClassApplication {
		return nil, NewParseError(opStartOffset, "protocolOp must have APPLICATION tag class", ErrInvalidOperation)
	}

	// Read the operation length
	opLength, err := decoder.ReadLength()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read protocolOp length", err)
	}

	// Current offset is at the start of operation content
	opContentStart := decoder.Offset()
	opContentEnd := opContentStart + opLength

	// Verify we have enough data
	if opContentEnd > len(data) {
		return nil, NewParseError(opContentStart, "truncated protocolOp data", ber.ErrUnexpectedEOF)
	}

	// Extract the operation content data directly from the original data slice
	opData := make([]byte, opLength)
	copy(opData, data[opContentStart:opContentEnd])

	// Create the raw operation
	rawOp := &RawOperation{
		Tag:  tagNum,
		Data: opData,
	}

	// Store whether operation is constructed for potential future use
	_ = constructed

	msg := &LDAPMessage{
		MessageID: int(msgID),
		Operation: rawOp,
	}

	// Check if there are controls (optional, context tag [0])
	// Controls start after the operation content
	if opContentEnd < seqContentEnd {
		// Create a new decoder for the remaining data
		remainingData := data[opContentEnd:seqContentEnd]
		if len(remainingData) > 0 {
			controlsDecoder := ber.NewBERDecoder(remainingData)

			// Check if next element is context tag [0] for controls
			if controlsDecoder.IsContextTag(ContextTagControls) {
				controls, err := parseControls(controlsDecoder)
				if err != nil {
					return nil, NewParseError(opContentEnd, "failed to parse controls", err)
				}
				msg.Controls = controls
			}
		}
	}

	return msg, nil
}

// parseControls parses the Controls field from the decoder.
// Controls ::= SEQUENCE OF control Control
func parseControls(decoder *ber.BERDecoder) ([]Control, error) {
	// Read context tag [0]
	ctrlSeqLength, err := decoder.ExpectContextTag(ContextTagControls)
	if err != nil {
		return nil, err
	}

	if ctrlSeqLength == 0 {
		return nil, nil
	}

	// Check what comes next - it could be:
	// 1. A SEQUENCE OF Control (standard)
	// 2. Direct Control SEQUENCE(s) (some clients)
	class, _, tagNum, peekErr := decoder.PeekTag()
	if peekErr != nil {
		return nil, peekErr
	}

	var controls []Control

	// If it's a SEQUENCE, check if it's a wrapper SEQUENCE OF or a Control SEQUENCE
	if class == ber.ClassUniversal && tagNum == ber.TagSequence {
		// Save position to potentially re-read
		startOffset := decoder.Offset()

		// Try to read as SEQUENCE OF Control first
		seqLength, err := decoder.ExpectSequence()
		if err != nil {
			return nil, err
		}

		seqEnd := decoder.Offset() + seqLength

		// Check if the first element inside is an OCTET STRING (OID) or another SEQUENCE
		if decoder.Remaining() > 0 {
			innerClass, _, innerTag, _ := decoder.PeekTag()

			if innerClass == ber.ClassUniversal && innerTag == ber.TagOctetString {
				// This SEQUENCE is a Control, not a wrapper
				// Reset and parse as single Control
				decoder.SetOffset(startOffset)
				ctrl, err := parseControl(decoder)
				if err != nil {
					return nil, err
				}
				controls = append(controls, ctrl)

				// Parse remaining controls
				for decoder.Remaining() > 0 {
					ctrl, err := parseControl(decoder)
					if err != nil {
						break
					}
					controls = append(controls, ctrl)
				}
				return controls, nil
			}
		}

		// It's a wrapper SEQUENCE OF Control
		for decoder.Offset() < seqEnd && decoder.Remaining() > 0 {
			ctrl, err := parseControl(decoder)
			if err != nil {
				return nil, err
			}
			controls = append(controls, ctrl)
		}
	} else {
		// Not a SEQUENCE, unexpected
		return nil, NewParseError(decoder.Offset(), "expected SEQUENCE for controls", nil)
	}

	return controls, nil
}

// parseControl parses a single Control from the decoder.
// Control ::= SEQUENCE {
//
//	controlType             LDAPOID,
//	criticality             BOOLEAN DEFAULT FALSE,
//	controlValue            OCTET STRING OPTIONAL
//
// }
func parseControl(decoder *ber.BERDecoder) (Control, error) {
	ctrl := Control{
		Criticality: false, // DEFAULT FALSE
	}

	// Read the Control SEQUENCE
	ctrlSeqDecoder, err := decoder.ReadSequenceContents()
	if err != nil {
		return ctrl, err
	}

	// Read controlType (LDAPOID - encoded as OCTET STRING)
	oidBytes, err := ctrlSeqDecoder.ReadOctetString()
	if err != nil {
		return ctrl, NewParseError(ctrlSeqDecoder.Offset(), "failed to read control OID", err)
	}
	ctrl.OID = string(oidBytes)

	// Check for optional criticality (BOOLEAN)
	if ctrlSeqDecoder.Remaining() > 0 {
		class, _, tagNum, err := ctrlSeqDecoder.PeekTag()
		if err == nil && class == ber.ClassUniversal && tagNum == ber.TagBoolean {
			criticality, err := ctrlSeqDecoder.ReadBoolean()
			if err != nil {
				return ctrl, NewParseError(ctrlSeqDecoder.Offset(), "failed to read control criticality", err)
			}
			ctrl.Criticality = criticality
		}
	}

	// Check for optional controlValue (OCTET STRING)
	if ctrlSeqDecoder.Remaining() > 0 {
		class, _, tagNum, err := ctrlSeqDecoder.PeekTag()
		if err == nil && class == ber.ClassUniversal && tagNum == ber.TagOctetString {
			value, err := ctrlSeqDecoder.ReadOctetString()
			if err != nil {
				return ctrl, NewParseError(ctrlSeqDecoder.Offset(), "failed to read control value", err)
			}
			ctrl.Value = value
		}
	}

	return ctrl, nil
}

// Encode encodes the LDAPMessage to BER format.
func (m *LDAPMessage) Encode() ([]byte, error) {
	// Validate message ID
	if m.MessageID < MinMessageID || m.MessageID > MaxMessageID {
		return nil, ErrInvalidMessageID
	}

	// Validate operation
	if m.Operation == nil {
		return nil, ErrMissingOperation
	}

	encoder := ber.NewBEREncoder(256)

	// Start the outer SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write messageID (INTEGER)
	if err := encoder.WriteInteger(int64(m.MessageID)); err != nil {
		return nil, err
	}

	// Write protocolOp (APPLICATION tagged)
	// Determine if the operation is constructed based on the tag
	// Most LDAP operations are constructed (SEQUENCE-like)
	constructed := isConstructedOperation(m.Operation.Tag)
	appPos := encoder.WriteApplicationTag(m.Operation.Tag, constructed)
	encoder.WriteRaw(m.Operation.Data)
	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	// Write controls if present
	if len(m.Controls) > 0 {
		if err := encodeControls(encoder, m.Controls); err != nil {
			return nil, err
		}
	}

	// End the outer SEQUENCE
	if err := encoder.EndSequence(seqPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// isConstructedOperation returns true if the operation type is constructed
func isConstructedOperation(tag int) bool {
	switch tag {
	case ApplicationUnbindRequest:
		// UnbindRequest is NULL (primitive)
		return false
	case ApplicationAbandonRequest:
		// AbandonRequest is INTEGER (primitive)
		return false
	case ApplicationDelRequest:
		// DelRequest is LDAPDN (OCTET STRING, primitive)
		return false
	default:
		// Most operations are SEQUENCE (constructed)
		return true
	}
}

// encodeControls encodes the Controls field.
func encodeControls(encoder *ber.BEREncoder, controls []Control) error {
	// Write context tag [0] for controls
	ctxPos := encoder.WriteContextTag(ContextTagControls, true)

	// Write SEQUENCE OF Control
	seqPos := encoder.BeginSequence()

	for _, ctrl := range controls {
		if err := encodeControl(encoder, ctrl); err != nil {
			return err
		}
	}

	if err := encoder.EndSequence(seqPos); err != nil {
		return err
	}

	return encoder.EndContextTag(ctxPos)
}

// encodeControl encodes a single Control.
func encodeControl(encoder *ber.BEREncoder, ctrl Control) error {
	// Start Control SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write controlType (LDAPOID as OCTET STRING)
	if err := encoder.WriteOctetString([]byte(ctrl.OID)); err != nil {
		return err
	}

	// Write criticality if true (omit if false since it's the default)
	if ctrl.Criticality {
		if err := encoder.WriteBoolean(true); err != nil {
			return err
		}
	}

	// Write controlValue if present
	if len(ctrl.Value) > 0 {
		if err := encoder.WriteOctetString(ctrl.Value); err != nil {
			return err
		}
	}

	return encoder.EndSequence(seqPos)
}
