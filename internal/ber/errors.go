// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding
// as specified in ITU-T X.690.
package ber

import (
	"errors"
	"fmt"
)

// Decoder errors
var (
	// ErrUnexpectedEOF is returned when the decoder encounters truncated data.
	ErrUnexpectedEOF = errors.New("ber: unexpected end of data")

	// ErrInvalidLength is returned when a length value is malformed.
	ErrInvalidLength = errors.New("ber: invalid length encoding")

	// ErrIndefiniteLength is returned when indefinite length encoding is encountered
	// but not supported for the current operation.
	ErrIndefiniteLength = errors.New("ber: indefinite length not supported")

	// ErrInvalidBoolean is returned when a boolean value has invalid length.
	ErrInvalidBoolean = errors.New("ber: invalid boolean encoding")

	// ErrInvalidInteger is returned when an integer value is malformed.
	ErrInvalidInteger = errors.New("ber: invalid integer encoding")

	// ErrInvalidNull is returned when a null value has non-zero length.
	ErrInvalidNull = errors.New("ber: invalid null encoding")

	// ErrTagMismatch is returned when the expected tag does not match the actual tag.
	ErrTagMismatch = errors.New("ber: tag mismatch")
)

// DecodeError provides detailed information about a decoding failure.
type DecodeError struct {
	Offset  int    // Byte offset where the error occurred
	Message string // Human-readable error description
	Err     error  // Underlying error
}

// Error implements the error interface.
func (e *DecodeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ber: decode error at offset %d: %s: %v", e.Offset, e.Message, e.Err)
	}
	return fmt.Sprintf("ber: decode error at offset %d: %s", e.Offset, e.Message)
}

// Unwrap returns the underlying error.
func (e *DecodeError) Unwrap() error {
	return e.Err
}

// NewDecodeError creates a new DecodeError with the given parameters.
func NewDecodeError(offset int, message string, err error) *DecodeError {
	return &DecodeError{
		Offset:  offset,
		Message: message,
		Err:     err,
	}
}

// TagMismatchError provides detailed information about a tag mismatch.
type TagMismatchError struct {
	Offset           int
	ExpectedClass    int
	ExpectedNumber   int
	ActualClass      int
	ActualNumber     int
	ActualConstructed int
}

// Error implements the error interface.
func (e *TagMismatchError) Error() string {
	return fmt.Sprintf("ber: tag mismatch at offset %d: expected class=%d number=%d, got class=%d number=%d constructed=%d",
		e.Offset, e.ExpectedClass, e.ExpectedNumber, e.ActualClass, e.ActualNumber, e.ActualConstructed)
}

// Is allows TagMismatchError to match ErrTagMismatch with errors.Is.
func (e *TagMismatchError) Is(target error) bool {
	return target == ErrTagMismatch
}
