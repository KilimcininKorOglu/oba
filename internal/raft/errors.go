package raft

import "errors"

// Raft errors.
var (
	// ErrNotLeader is returned when a write operation is attempted on a non-leader node.
	ErrNotLeader = errors.New("raft: not the leader")

	// ErrLeaderUnknown is returned when the leader is not known.
	ErrLeaderUnknown = errors.New("raft: leader unknown")

	// ErrNodeStopped is returned when operation is attempted on a stopped node.
	ErrNodeStopped = errors.New("raft: node stopped")

	// ErrLogCorrupted is returned when log data is corrupted.
	ErrLogCorrupted = errors.New("raft: log corrupted")

	// ErrLogIndexOutOfRange is returned when accessing an invalid log index.
	ErrLogIndexOutOfRange = errors.New("raft: log index out of range")

	// ErrSnapshotFailed is returned when snapshot creation fails.
	ErrSnapshotFailed = errors.New("raft: snapshot failed")

	// ErrRestoreFailed is returned when snapshot restore fails.
	ErrRestoreFailed = errors.New("raft: restore failed")

	// ErrTransportClosed is returned when transport is closed.
	ErrTransportClosed = errors.New("raft: transport closed")

	// ErrConnectFailed is returned when connection to peer fails.
	ErrConnectFailed = errors.New("raft: connection failed")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("raft: operation timeout")

	// ErrInvalidConfig is returned when configuration is invalid.
	ErrInvalidConfig = errors.New("raft: invalid configuration")
)
