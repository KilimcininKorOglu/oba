package logging

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ArchiveConfig holds configuration for log archiving.
type ArchiveConfig struct {
	Enabled    bool          // Enable archiving
	ArchiveDir string        // Directory for archive files
	MaxAge     time.Duration // Max age before archiving (e.g., 7 days)
	MaxSize    int64         // Max active log size in bytes before archiving
	Compress   bool          // Compress archive files with gzip
	RetainDays int           // Days to retain archives (0 = forever)
}

// DefaultArchiveConfig returns default archive configuration.
func DefaultArchiveConfig() ArchiveConfig {
	return ArchiveConfig{
		Enabled:    false,
		MaxAge:     7 * 24 * time.Hour, // 7 days
		MaxSize:    100 * 1024 * 1024,  // 100MB
		Compress:   true,
		RetainDays: 0, // Keep forever
	}
}

// LogArchive manages archived log files.
type LogArchive struct {
	config ArchiveConfig
	mu     sync.RWMutex
}

// NewLogArchive creates a new log archive manager.
func NewLogArchive(config ArchiveConfig) (*LogArchive, error) {
	if !config.Enabled {
		return nil, nil
	}

	if config.ArchiveDir == "" {
		return nil, fmt.Errorf("archive directory required")
	}

	// Create archive directory if not exists
	if err := os.MkdirAll(config.ArchiveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	return &LogArchive{config: config}, nil
}

// ArchiveFile represents a single archive file.
type ArchiveFile struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Count      int       `json:"count"`
	Compressed bool      `json:"compressed"`
}

// Archive writes log entries to an archive file.
func (a *LogArchive) Archive(entries []LogEntry) (*ArchiveFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(entries) == 0 {
		return nil, nil
	}

	// Sort entries by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	startTime := entries[0].Timestamp
	endTime := entries[len(entries)-1].Timestamp

	// Generate filename: logs_YYYYMMDD_HHMMSS_YYYYMMDD_HHMMSS.json[.gz]
	filename := fmt.Sprintf("logs_%s_%s.json",
		startTime.Format("20060102_150405"),
		endTime.Format("20060102_150405"))

	if a.config.Compress {
		filename += ".gz"
	}

	filePath := filepath.Join(a.config.ArchiveDir, filename)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file

	// Wrap with gzip if compression enabled
	if a.config.Compress {
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	}

	// Write entries as JSON lines
	encoder := json.NewEncoder(writer)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return nil, fmt.Errorf("failed to write entry: %w", err)
		}
	}

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return &ArchiveFile{
		Name:       filename,
		Path:       filePath,
		Size:       info.Size(),
		StartTime:  startTime,
		EndTime:    endTime,
		Count:      len(entries),
		Compressed: a.config.Compress,
	}, nil
}

// ListArchives returns all archive files.
func (a *LogArchive) ListArchives() ([]ArchiveFile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var archives []ArchiveFile

	entries, err := os.ReadDir(a.config.ArchiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return archives, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "logs_") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		archive := ArchiveFile{
			Name:       name,
			Path:       filepath.Join(a.config.ArchiveDir, name),
			Size:       info.Size(),
			Compressed: strings.HasSuffix(name, ".gz"),
		}

		// Parse timestamps from filename
		archive.StartTime, archive.EndTime = parseArchiveFilename(name)

		// Count entries (expensive, skip for listing)
		archives = append(archives, archive)
	}

	// Sort by start time descending (newest first)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].StartTime.After(archives[j].StartTime)
	})

	return archives, nil
}

// parseArchiveFilename extracts timestamps from archive filename.
func parseArchiveFilename(name string) (start, end time.Time) {
	// Format: logs_YYYYMMDD_HHMMSS_YYYYMMDD_HHMMSS.json[.gz]
	name = strings.TrimPrefix(name, "logs_")
	name = strings.TrimSuffix(name, ".gz")
	name = strings.TrimSuffix(name, ".json")

	parts := strings.Split(name, "_")
	if len(parts) >= 4 {
		start, _ = time.Parse("20060102150405", parts[0]+parts[1])
		end, _ = time.Parse("20060102150405", parts[2]+parts[3])
	}
	return
}

// QueryArchive searches entries in a specific archive file.
func (a *LogArchive) QueryArchive(archivePath string, opts QueryOptions) ([]LogEntry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file

	// Decompress if gzipped
	if strings.HasSuffix(archivePath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var entries []LogEntry
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		// Apply filters
		if !matchesFilter(entry, opts) {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// QueryAllArchives searches entries across all archive files within time range.
func (a *LogArchive) QueryAllArchives(opts QueryOptions) ([]LogEntry, int, error) {
	archives, err := a.ListArchives()
	if err != nil {
		return nil, 0, err
	}

	var allEntries []LogEntry

	for _, archive := range archives {
		// Skip archives outside time range
		if !opts.StartTime.IsZero() && archive.EndTime.Before(opts.StartTime) {
			continue
		}
		if !opts.EndTime.IsZero() && archive.StartTime.After(opts.EndTime) {
			continue
		}

		entries, err := a.QueryArchive(archive.Path, opts)
		if err != nil {
			continue // Skip failed archives
		}

		allEntries = append(allEntries, entries...)
	}

	// Sort by timestamp descending
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
	})

	total := len(allEntries)

	// Apply pagination
	if opts.Offset > 0 && opts.Offset < len(allEntries) {
		allEntries = allEntries[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(allEntries) {
		allEntries = allEntries[:opts.Limit]
	}

	return allEntries, total, nil
}

// matchesFilter checks if entry matches query options.
func matchesFilter(entry LogEntry, opts QueryOptions) bool {
	// Level filter
	if opts.Level != "" && entry.Level != opts.Level {
		return false
	}

	// Source filter
	if opts.Source != "" && entry.Source != opts.Source {
		return false
	}

	// User filter
	if opts.User != "" && entry.User != opts.User {
		return false
	}

	// Time range filter
	if !opts.StartTime.IsZero() && entry.Timestamp.Before(opts.StartTime) {
		return false
	}
	if !opts.EndTime.IsZero() && entry.Timestamp.After(opts.EndTime) {
		return false
	}

	// Search filter
	if opts.Search != "" {
		search := strings.ToLower(opts.Search)
		if !strings.Contains(strings.ToLower(entry.Message), search) &&
			!strings.Contains(strings.ToLower(entry.Source), search) &&
			!strings.Contains(strings.ToLower(entry.User), search) {
			return false
		}
	}

	return true
}

// CleanupOldArchives removes archives older than retention period.
func (a *LogArchive) CleanupOldArchives() (int, error) {
	if a.config.RetainDays <= 0 {
		return 0, nil // Keep forever
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -a.config.RetainDays)
	deleted := 0

	archives, err := a.ListArchives()
	if err != nil {
		return 0, err
	}

	for _, archive := range archives {
		if archive.EndTime.Before(cutoff) {
			if err := os.Remove(archive.Path); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// GetArchiveStats returns statistics about archives.
func (a *LogArchive) GetArchiveStats() ArchiveStats {
	archives, _ := a.ListArchives()

	stats := ArchiveStats{
		ArchiveCount: len(archives),
	}

	for _, archive := range archives {
		stats.TotalSize += archive.Size
		stats.TotalEntries += archive.Count

		if stats.OldestArchive.IsZero() || archive.StartTime.Before(stats.OldestArchive) {
			stats.OldestArchive = archive.StartTime
		}
		if archive.EndTime.After(stats.NewestArchive) {
			stats.NewestArchive = archive.EndTime
		}
	}

	return stats
}

// ArchiveStats contains statistics about log archives.
type ArchiveStats struct {
	ArchiveCount  int       `json:"archive_count"`
	TotalSize     int64     `json:"total_size"`
	TotalEntries  int       `json:"total_entries"`
	OldestArchive time.Time `json:"oldest_archive,omitempty"`
	NewestArchive time.Time `json:"newest_archive,omitempty"`
}
