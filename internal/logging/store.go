package logging

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/engine"
)

// LogEntry represents a single log entry stored in the database.
type LogEntry struct {
	ID        uint64                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogStore provides persistent storage for log entries using ObaDB.
type LogStore struct {
	mu         sync.RWMutex
	db         *engine.ObaDB
	nextID     uint64
	maxEntries int
	dbPath     string
}

// LogStoreConfig holds configuration for the log store.
type LogStoreConfig struct {
	Enabled    bool
	DBPath     string
	MaxEntries int
}

// NewLogStore creates a new log store with the given configuration.
func NewLogStore(cfg LogStoreConfig) (*LogStore, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	maxEntries := cfg.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 100000
	}

	opts := storage.EngineOptions{
		PageSize:          4096,
		BufferPoolSize:    64,
		InitialPages:      16,
		CreateIfNotExists: true,
		SyncOnWrite:       false,
	}

	db, err := engine.Open(cfg.DBPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open log database: %w", err)
	}

	store := &LogStore{
		db:         db,
		maxEntries: maxEntries,
		dbPath:     cfg.DBPath,
		nextID:     1,
	}

	// Load next ID from existing entries
	if err := store.loadNextID(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// loadNextID finds the highest existing ID to continue from.
func (s *LogStore) loadNextID() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer s.db.Rollback(tx)

	// Search for all log entries
	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	defer iter.Close()

	var maxID uint64
	for iter.Next() {
		entry := iter.Entry()
		if idVals := entry.GetAttribute("logid"); len(idVals) > 0 {
			if id, err := strconv.ParseUint(string(idVals[0]), 10, 64); err == nil {
				if id > maxID {
					maxID = id
				}
			}
		}
	}

	s.nextID = maxID + 1
	return nil
}

// Write adds a new log entry to the store.
func (s *LogStore) Write(level, msg, requestID string, fields map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}

	id := s.nextID
	s.nextID++

	entry := &storage.Entry{
		DN:         fmt.Sprintf("id=%d,ou=logs", id),
		Attributes: make(map[string][][]byte),
	}

	entry.SetStringAttribute("objectclass", "logEntry")
	entry.SetStringAttribute("logid", strconv.FormatUint(id, 10))
	entry.SetStringAttribute("timestamp", time.Now().UTC().Format(time.RFC3339Nano))
	entry.SetStringAttribute("level", level)
	entry.SetStringAttribute("message", msg)

	if requestID != "" {
		entry.SetStringAttribute("requestid", requestID)
	}

	if len(fields) > 0 {
		fieldsJSON, _ := json.Marshal(fields)
		entry.SetStringAttribute("fields", string(fieldsJSON))
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	// Ensure parent exists
	parentDN := "ou=logs"
	if _, err := s.db.Get(tx, parentDN); err != nil {
		parent := &storage.Entry{
			DN:         parentDN,
			Attributes: make(map[string][][]byte),
		}
		parent.SetStringAttribute("objectclass", "organizationalUnit")
		parent.SetStringAttribute("ou", "logs")
		if err := s.db.Put(tx, parent); err != nil {
			s.db.Rollback(tx)
			return err
		}
	}

	if err := s.db.Put(tx, entry); err != nil {
		s.db.Rollback(tx)
		return err
	}

	if err := s.db.Commit(tx); err != nil {
		return err
	}

	// Trim old entries if needed (async)
	go s.trimOldEntries()

	return nil
}

// trimOldEntries removes oldest entries if we exceed max.
func (s *LogStore) trimOldEntries() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer s.db.Rollback(tx)

	// Count entries
	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	var entries []struct {
		dn string
		id uint64
	}

	for iter.Next() {
		entry := iter.Entry()
		if entry.DN == "ou=logs" {
			continue
		}
		var id uint64
		if idVals := entry.GetAttribute("logid"); len(idVals) > 0 {
			id, _ = strconv.ParseUint(string(idVals[0]), 10, 64)
		}
		entries = append(entries, struct {
			dn string
			id uint64
		}{entry.DN, id})
	}
	iter.Close()

	if len(entries) <= s.maxEntries {
		return
	}

	// Sort by ID and delete oldest
	deleteCount := len(entries) - s.maxEntries
	if deleteCount > 0 {
		// Simple bubble sort for small deletions
		for i := 0; i < deleteCount; i++ {
			minIdx := i
			for j := i + 1; j < len(entries); j++ {
				if entries[j].id < entries[minIdx].id {
					minIdx = j
				}
			}
			entries[i], entries[minIdx] = entries[minIdx], entries[i]
		}

		tx2, err := s.db.Begin()
		if err != nil {
			return
		}

		for i := 0; i < deleteCount; i++ {
			s.db.Delete(tx2, entries[i].dn)
		}

		s.db.Commit(tx2)
	}
}

// Query searches log entries with the given filters.
func (s *LogStore) Query(opts QueryOptions) ([]LogEntry, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return nil, 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer s.db.Rollback(tx)

	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	defer iter.Close()

	var allEntries []LogEntry

	for iter.Next() {
		entry := iter.Entry()
		if entry.DN == "ou=logs" {
			continue
		}

		logEntry := s.entryToLogEntry(entry)

		// Apply filters
		if opts.Level != "" && logEntry.Level != opts.Level {
			continue
		}
		if opts.RequestID != "" && logEntry.RequestID != opts.RequestID {
			continue
		}
		if !opts.StartTime.IsZero() && logEntry.Timestamp.Before(opts.StartTime) {
			continue
		}
		if !opts.EndTime.IsZero() && logEntry.Timestamp.After(opts.EndTime) {
			continue
		}
		if opts.Search != "" && !strings.Contains(strings.ToLower(logEntry.Message), strings.ToLower(opts.Search)) {
			continue
		}

		allEntries = append(allEntries, logEntry)
	}

	// Sort by ID descending (newest first)
	for i := 0; i < len(allEntries)-1; i++ {
		for j := i + 1; j < len(allEntries); j++ {
			if allEntries[j].ID > allEntries[i].ID {
				allEntries[i], allEntries[j] = allEntries[j], allEntries[i]
			}
		}
	}

	total := len(allEntries)

	// Apply pagination
	if opts.Offset > 0 {
		if opts.Offset >= len(allEntries) {
			allEntries = nil
		} else {
			allEntries = allEntries[opts.Offset:]
		}
	}

	if opts.Limit > 0 && len(allEntries) > opts.Limit {
		allEntries = allEntries[:opts.Limit]
	}

	return allEntries, total, nil
}

// entryToLogEntry converts a storage entry to a LogEntry.
func (s *LogStore) entryToLogEntry(entry *storage.Entry) LogEntry {
	var logEntry LogEntry

	if idVals := entry.GetAttribute("logid"); len(idVals) > 0 {
		logEntry.ID, _ = strconv.ParseUint(string(idVals[0]), 10, 64)
	}
	if tsVals := entry.GetAttribute("timestamp"); len(tsVals) > 0 {
		logEntry.Timestamp, _ = time.Parse(time.RFC3339Nano, string(tsVals[0]))
	}
	if lvlVals := entry.GetAttribute("level"); len(lvlVals) > 0 {
		logEntry.Level = string(lvlVals[0])
	}
	if msgVals := entry.GetAttribute("message"); len(msgVals) > 0 {
		logEntry.Message = string(msgVals[0])
	}
	if reqVals := entry.GetAttribute("requestid"); len(reqVals) > 0 {
		logEntry.RequestID = string(reqVals[0])
	}
	if fieldVals := entry.GetAttribute("fields"); len(fieldVals) > 0 {
		json.Unmarshal(fieldVals[0], &logEntry.Fields)
	}

	return logEntry
}

// QueryOptions defines filters for querying logs.
type QueryOptions struct {
	Level     string
	RequestID string
	StartTime time.Time
	EndTime   time.Time
	Search    string
	Offset    int
	Limit     int
}

// GetStats returns statistics about the log store.
func (s *LogStore) GetStats() LogStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := LogStoreStats{
		MaxEntries: s.maxEntries,
		ByLevel:    make(map[string]int),
	}

	if s.db == nil {
		return stats
	}

	tx, err := s.db.Begin()
	if err != nil {
		return stats
	}
	defer s.db.Rollback(tx)

	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	defer iter.Close()

	var oldest, newest time.Time
	for iter.Next() {
		entry := iter.Entry()
		if entry.DN == "ou=logs" {
			continue
		}

		stats.TotalEntries++

		if lvlVals := entry.GetAttribute("level"); len(lvlVals) > 0 {
			stats.ByLevel[string(lvlVals[0])]++
		}

		if tsVals := entry.GetAttribute("timestamp"); len(tsVals) > 0 {
			if ts, err := time.Parse(time.RFC3339Nano, string(tsVals[0])); err == nil {
				if oldest.IsZero() || ts.Before(oldest) {
					oldest = ts
				}
				if newest.IsZero() || ts.After(newest) {
					newest = ts
				}
			}
		}
	}

	stats.OldestEntry = oldest
	stats.NewestEntry = newest

	return stats
}

// LogStoreStats contains statistics about the log store.
type LogStoreStats struct {
	TotalEntries int
	MaxEntries   int
	ByLevel      map[string]int
	OldestEntry  time.Time
	NewestEntry  time.Time
}

// Clear removes all log entries.
func (s *LogStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	var dns []string
	for iter.Next() {
		entry := iter.Entry()
		if entry.DN != "ou=logs" {
			dns = append(dns, entry.DN)
		}
	}
	iter.Close()
	s.db.Rollback(tx)

	tx2, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, dn := range dns {
		s.db.Delete(tx2, dn)
	}

	if err := s.db.Commit(tx2); err != nil {
		return err
	}

	s.nextID = 1
	return nil
}

// Close closes the log store and releases resources.
func (s *LogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// Export exports logs in the specified format.
func (s *LogStore) Export(format string, opts QueryOptions) ([]byte, error) {
	entries, _, err := s.Query(opts)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return json.MarshalIndent(entries, "", "  ")
	case "jsonl":
		var lines []string
		for _, entry := range entries {
			line, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			lines = append(lines, string(line))
		}
		return []byte(strings.Join(lines, "\n")), nil
	case "csv":
		return s.exportCSV(entries), nil
	default:
		return json.Marshal(entries)
	}
}

// exportCSV exports entries as CSV.
func (s *LogStore) exportCSV(entries []LogEntry) []byte {
	var lines []string
	lines = append(lines, "id,timestamp,level,message,request_id")

	for _, entry := range entries {
		line := fmt.Sprintf("%d,%s,%s,%s,%s",
			entry.ID,
			entry.Timestamp.Format(time.RFC3339),
			entry.Level,
			strconv.Quote(entry.Message),
			entry.RequestID,
		)
		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n"))
}
