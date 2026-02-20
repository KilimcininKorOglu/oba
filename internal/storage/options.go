// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"time"
)

// EngineOptions configures the ObaDB storage engine.
type EngineOptions struct {
	// DataDir is the directory where database files are stored.
	DataDir string

	// PageSize is the size of each page in bytes.
	// Default: 4096 bytes.
	PageSize int

	// BufferPoolSize is the number of pages to cache in memory.
	// Default: 256 pages.
	BufferPoolSize int

	// WALBufferSize is the size of the WAL write buffer in bytes.
	// Default: 64KB.
	WALBufferSize int

	// SyncOnWrite forces fsync after each write operation.
	// Default: false (better performance, less durability).
	SyncOnWrite bool

	// ReadOnly opens the database in read-only mode.
	// Default: false.
	ReadOnly bool

	// CreateIfNotExists creates the database if it doesn't exist.
	// Default: true.
	CreateIfNotExists bool

	// CheckpointInterval is the time between automatic checkpoints.
	// Default: 5 minutes.
	CheckpointInterval time.Duration

	// GCInterval is the time between automatic garbage collection runs.
	// Default: 30 seconds.
	GCInterval time.Duration

	// GCEnabled enables automatic garbage collection.
	// Default: true.
	GCEnabled bool

	// MaxOpenFiles is the maximum number of open file descriptors.
	// Default: 1000.
	MaxOpenFiles int

	// InitialPages is the initial number of pages to allocate.
	// Default: 16.
	InitialPages int

	// EncryptionKeyFile is the path to the encryption key file.
	// If empty, encryption is disabled.
	EncryptionKeyFile string

	// EncryptionKey is the raw encryption key (32 bytes).
	// If set, takes precedence over EncryptionKeyFile.
	// Warning: Prefer EncryptionKeyFile for production use.
	EncryptionKey []byte
}

// DefaultEngineOptions returns the default engine options.
func DefaultEngineOptions() EngineOptions {
	return EngineOptions{
		DataDir:            ".",
		PageSize:           PageSize,
		BufferPoolSize:     256,
		WALBufferSize:      64 * 1024,
		SyncOnWrite:        false,
		ReadOnly:           false,
		CreateIfNotExists:  true,
		CheckpointInterval: 5 * time.Minute,
		GCInterval:         30 * time.Second,
		GCEnabled:          true,
		MaxOpenFiles:       1000,
		InitialPages:       16,
	}
}

// Validate validates the engine options and returns an error if invalid.
func (o *EngineOptions) Validate() error {
	if o.PageSize <= 0 {
		o.PageSize = PageSize
	}

	if o.BufferPoolSize <= 0 {
		o.BufferPoolSize = 256
	}

	if o.WALBufferSize <= 0 {
		o.WALBufferSize = 64 * 1024
	}

	if o.CheckpointInterval <= 0 {
		o.CheckpointInterval = 5 * time.Minute
	}

	if o.GCInterval <= 0 {
		o.GCInterval = 30 * time.Second
	}

	if o.InitialPages <= 0 {
		o.InitialPages = 16
	}

	return nil
}

// WithDataDir sets the data directory.
func (o EngineOptions) WithDataDir(dir string) EngineOptions {
	o.DataDir = dir
	return o
}

// WithPageSize sets the page size.
func (o EngineOptions) WithPageSize(size int) EngineOptions {
	o.PageSize = size
	return o
}

// WithBufferPoolSize sets the buffer pool size.
func (o EngineOptions) WithBufferPoolSize(size int) EngineOptions {
	o.BufferPoolSize = size
	return o
}

// WithSyncOnWrite enables or disables sync on write.
func (o EngineOptions) WithSyncOnWrite(sync bool) EngineOptions {
	o.SyncOnWrite = sync
	return o
}

// WithReadOnly enables or disables read-only mode.
func (o EngineOptions) WithReadOnly(readOnly bool) EngineOptions {
	o.ReadOnly = readOnly
	return o
}

// WithCreateIfNotExists enables or disables auto-creation.
func (o EngineOptions) WithCreateIfNotExists(create bool) EngineOptions {
	o.CreateIfNotExists = create
	return o
}

// WithCheckpointInterval sets the checkpoint interval.
func (o EngineOptions) WithCheckpointInterval(interval time.Duration) EngineOptions {
	o.CheckpointInterval = interval
	return o
}

// WithGCInterval sets the garbage collection interval.
func (o EngineOptions) WithGCInterval(interval time.Duration) EngineOptions {
	o.GCInterval = interval
	return o
}

// WithGCEnabled enables or disables garbage collection.
func (o EngineOptions) WithGCEnabled(enabled bool) EngineOptions {
	o.GCEnabled = enabled
	return o
}

// WithEncryptionKeyFile sets the encryption key file path.
func (o EngineOptions) WithEncryptionKeyFile(path string) EngineOptions {
	o.EncryptionKeyFile = path
	return o
}

// WithEncryptionKey sets the raw encryption key.
func (o EngineOptions) WithEncryptionKey(key []byte) EngineOptions {
	o.EncryptionKey = key
	return o
}
