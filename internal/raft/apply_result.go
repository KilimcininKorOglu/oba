package raft

import (
	"errors"
	"fmt"
)

// ApplyResult describes how a committed command affected the local state machine.
type ApplyResult uint8

const (
	ApplyResultApplied ApplyResult = iota
	ApplyResultIdempotent
	ApplyResultRejectConflict
	ApplyResultFatal
)

func (r ApplyResult) String() string {
	switch r {
	case ApplyResultApplied:
		return "applied"
	case ApplyResultIdempotent:
		return "idempotent"
	case ApplyResultRejectConflict:
		return "reject_conflict"
	case ApplyResultFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ApplyResultError wraps an apply error with deterministic semantics for applyLoop.
type ApplyResultError struct {
	Result ApplyResult
	Err    error
}

func (e *ApplyResultError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("apply result: %s", e.Result.String())
}

func (e *ApplyResultError) Unwrap() error {
	return e.Err
}

// NewApplyResultError creates a typed apply error.
func NewApplyResultError(result ApplyResult, err error) error {
	return &ApplyResultError{
		Result: result,
		Err:    err,
	}
}

// ApplyResultFromError classifies an apply error.
// Unknown errors are always treated as fatal.
func ApplyResultFromError(err error) ApplyResult {
	if err == nil {
		return ApplyResultApplied
	}
	var typed *ApplyResultError
	if errors.As(err, &typed) {
		return typed.Result
	}
	return ApplyResultFatal
}
