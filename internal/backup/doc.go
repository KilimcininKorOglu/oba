// Package backup provides backup and restore functionality for ObaDB.
//
// # Overview
//
// The backup package implements database backup and restore operations for
// the ObaDB storage engine. It supports:
//
//   - Full database backups with consistent snapshots
//   - Incremental backups (changes since last backup)
//   - Compression for reduced storage
//   - LDIF export/import for interoperability
//   - Checksum verification for data integrity
//
// # Backup Formats
//
// Two backup formats are supported:
//
//   - Native: Binary format optimized for fast backup/restore
//   - LDIF: Text format for interoperability with other LDAP servers
//
// # Creating Backups
//
// Create a full backup:
//
//	opts := &backup.BackupOptions{
//	    OutputPath: "/backup/oba-20260218.bak",
//	    Compress:   true,
//	    Format:     backup.FormatNative,
//	}
//
//	stats, err := backup.Full(engine, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Backed up %d entries in %v\n", stats.EntryCount, stats.Duration)
//
// # LDIF Export
//
// Export to LDIF format:
//
//	opts := &backup.BackupOptions{
//	    OutputPath: "/backup/data.ldif",
//	    Format:     backup.FormatLDIF,
//	    BaseDN:     "dc=example,dc=com",
//	}
//
//	stats, err := backup.Full(engine, opts)
//
// # Restoring Backups
//
// Restore from a backup:
//
//	opts := &backup.RestoreOptions{
//	    InputPath: "/backup/oba-20260218.bak",
//	    Verify:    true, // Verify checksums before restore
//	    Format:    backup.FormatNative,
//	}
//
//	err := backup.Restore(engine, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # LDIF Import
//
// Import from LDIF:
//
//	opts := &backup.RestoreOptions{
//	    InputPath: "/backup/data.ldif",
//	    Format:    backup.FormatLDIF,
//	}
//
//	err := backup.Restore(engine, opts)
//
// # Backup Header
//
// Native backups include a header with metadata:
//
//	header := backup.NewBackupHeader()
//	header.PageSize = 4096
//	header.TotalPages = 1000
//	header.EntryCount = 5000
//	header.SetCompressed(true)
//
// # Backup Statistics
//
// BackupStats provides information about the backup:
//
//	stats := &backup.BackupStats{
//	    TotalPages:      1000,
//	    TotalBytes:      4096000,
//	    CompressedBytes: 1024000,
//	    Duration:        5 * time.Second,
//	    EntryCount:      5000,
//	}
//
//	ratio := stats.CompressionRatio() // 0.75 (75% reduction)
//
// # Error Handling
//
// Common backup errors:
//
//   - ErrInvalidBackup: Backup file is malformed
//   - ErrInvalidMagic: Not a valid ObaDB backup
//   - ErrChecksumMismatch: Data corruption detected
//   - ErrUnsupportedFormat: Unknown backup version
package backup
