package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
)

// LogEntry represents a single log entry stored in the database.
type LogEntry struct {
	ID        uint64                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source,omitempty"`
	User      string                 `json:"user,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogStore provides persistent storage for log entries using ObaDB.
type LogStore struct {
	mu            sync.RWMutex
	db            *engine.ObaDB
	clusterWriter ClusterWriter
	archive       *LogArchive
	nextID        uint64
	maxEntries    int
	maxAge        time.Duration
	dbPath        string
	archiveDir    string

	// Buffer for entries written before cluster writer is set
	pendingEntries []*storage.Entry
	clusterMode    bool // Flag to indicate cluster mode is enabled

	// Retry buffer for failed forwards
	retryBuffer   []*storage.Entry
	retryMu       sync.Mutex
	retryRunning  bool
	retryStopChan chan struct{}
}

// ClusterWriter interface for cluster-aware write operations.
type ClusterWriter interface {
	PutLog(entry *storage.Entry) error
	DeleteLog(dn string) error
	IsLeader() bool
	LeaderAddr() string
	NodeID() uint64
}

// LogStoreConfig holds configuration for the log store.
type LogStoreConfig struct {
	Enabled    bool
	DBPath     string
	MaxEntries int
	MaxAge     time.Duration // Max age before archiving
	ArchiveDir string        // Directory for archives (empty = no archiving)
	Compress   bool          // Compress archives
	RetainDays int           // Days to retain archives (0 = forever)
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
		maxAge:     cfg.MaxAge,
		dbPath:     cfg.DBPath,
		archiveDir: cfg.ArchiveDir,
		nextID:     1,
	}

	// Create archive manager if archive directory is configured
	if cfg.ArchiveDir != "" {
		archive, err := NewLogArchive(ArchiveConfig{
			Enabled:    true,
			ArchiveDir: cfg.ArchiveDir,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			RetainDays: cfg.RetainDays,
		})
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create archive: %w", err)
		}
		store.archive = archive
	}

	// Load next ID from existing entries
	if err := store.loadNextID(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// SetClusterWriter sets the cluster writer for cluster-aware write operations.
// It also flushes any pending entries that were buffered before the cluster writer was set.
func (s *LogStore) SetClusterWriter(cw ClusterWriter) {
	s.mu.Lock()
	pending := s.pendingEntries
	s.pendingEntries = nil
	s.clusterWriter = cw
	s.mu.Unlock()

	// Flush pending entries through cluster writer (async to wait for Raft to be ready)
	if len(pending) > 0 && cw != nil {
		go s.flushPendingEntries(cw, pending)
	}
}

// flushPendingEntries writes buffered entries to the cluster after Raft is ready.
func (s *LogStore) flushPendingEntries(cw ClusterWriter, pending []*storage.Entry) {
	// Wait for Raft to stabilize (leader election)
	time.Sleep(5 * time.Second)

	// Only leader can write to Raft, followers skip flushing
	// Their logs will not be persisted (this is acceptable for system startup logs)
	if !cw.IsLeader() {
		return
	}

	// Ensure parent exists first
	s.mu.Lock()
	tx, err := s.db.Begin()
	if err == nil {
		parentDN := "ou=logs"
		if _, err := s.db.Get(tx, parentDN); err != nil {
			parent := &storage.Entry{
				DN:         parentDN,
				Attributes: make(map[string][][]byte),
			}
			parent.SetStringAttribute("objectclass", "organizationalUnit")
			parent.SetStringAttribute("ou", "logs")
			cw.PutLog(parent)
		}
		s.db.Rollback(tx)
	}
	s.mu.Unlock()

	// Write pending entries
	for _, entry := range pending {
		cw.PutLog(entry)
	}
}

// EnableClusterMode enables cluster mode buffering for log entries.
// Call this before cluster writer is set to buffer early log entries.
func (s *LogStore) EnableClusterMode() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterMode = true
	s.pendingEntries = make([]*storage.Entry, 0, 100)
}

// isClusterMode returns true if cluster mode is enabled.
func (s *LogStore) isClusterMode() bool {
	return s.clusterMode
}

// PendingCount returns the number of pending entries (for debugging).
func (s *LogStore) PendingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pendingEntries)
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
func (s *LogStore) Write(level, msg, source, user, requestID string, fields map[string]interface{}) error {
	s.mu.Lock()

	if s.db == nil {
		s.mu.Unlock()
		return nil
	}

	// Skip Raft's own logs to prevent infinite loop:
	// Raft log -> LogStore.Write -> Propose -> Raft waits -> deadlock
	// Raft logs are written directly to stdout, not replicated
	if source == "raft" {
		s.mu.Unlock()
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

	if source != "" {
		entry.SetStringAttribute("source", source)
	}

	if user != "" {
		entry.SetStringAttribute("user", user)
	}

	if requestID != "" {
		entry.SetStringAttribute("requestid", requestID)
	}

	// Add nodeId to fields if in cluster mode and not already set
	cw := s.clusterWriter
	if cw != nil {
		if fields == nil {
			fields = make(map[string]interface{})
		}
		// Only set nodeId if not already present (preserve forwarded log's nodeId)
		if _, exists := fields["nodeId"]; !exists {
			fields["nodeId"] = cw.NodeID()
		}
	}

	if len(fields) > 0 {
		fieldsJSON, _ := json.Marshal(fields)
		entry.SetStringAttribute("fields", string(fieldsJSON))
	}

	// If cluster writer is set, route through Raft consensus
	if cw != nil {
		// Leader writes directly to Raft
		if cw.IsLeader() {
			err := s.writeViaRaft(entry)
			s.mu.Unlock()
			return err
		}

		// Follower: forward to leader via HTTP
		leaderAddr := cw.LeaderAddr()
		if leaderAddr == "" {
			// No leader yet, buffer the entry
			if len(s.pendingEntries) < 1000 {
				s.pendingEntries = append(s.pendingEntries, entry)
			}
			s.mu.Unlock()
			return nil
		}

		// Release lock before network I/O
		s.mu.Unlock()

		// Forward to leader (non-blocking relative to lock)
		return s.forwardToLeader(entry, leaderAddr)
	}

	// Standalone mode or cluster mode without cluster writer: direct write
	err := s.writeLocal(entry)
	s.mu.Unlock()
	return err
}

// writeViaRaft writes an entry through Raft consensus (leader only).
func (s *LogStore) writeViaRaft(entry *storage.Entry) error {
	// Ensure parent exists first (local check)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	parentDN := "ou=logs"
	if _, err := s.db.Get(tx, parentDN); err != nil {
		parent := &storage.Entry{
			DN:         parentDN,
			Attributes: make(map[string][][]byte),
		}
		parent.SetStringAttribute("objectclass", "organizationalUnit")
		parent.SetStringAttribute("ou", "logs")
		if err := s.clusterWriter.PutLog(parent); err != nil {
			s.db.Rollback(tx)
			return err
		}
	}
	s.db.Rollback(tx)

	// Write log entry through cluster
	if err := s.clusterWriter.PutLog(entry); err != nil {
		return err
	}

	// Trim old entries if needed (async)
	go s.trimOldEntries()
	return nil
}

// forwardToLeader forwards a log entry to the leader node via HTTP.
func (s *LogStore) forwardToLeader(entry *storage.Entry, leaderAddr string) error {
	// Extract log data from entry
	logEntry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     string(entry.GetAttribute("level")[0]),
		Message:   string(entry.GetAttribute("message")[0]),
	}

	if src := entry.GetAttribute("source"); len(src) > 0 {
		logEntry.Source = string(src[0])
	}
	if usr := entry.GetAttribute("user"); len(usr) > 0 {
		logEntry.User = string(usr[0])
	}
	if rid := entry.GetAttribute("requestid"); len(rid) > 0 {
		logEntry.RequestID = string(rid[0])
	}
	if fld := entry.GetAttribute("fields"); len(fld) > 0 {
		json.Unmarshal(fld[0], &logEntry.Fields)
	}

	// Convert leader Raft address to HTTP address
	// Raft addr format: "oba-node1:4445" -> HTTP: "oba-node1:8080"
	httpAddr := strings.Replace(leaderAddr, ":4445", ":8080", 1)

	// Send to leader's internal log endpoint
	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/api/v1/internal/log", httpAddr)

	// Retry up to 3 times with backoff
	for i := 0; i < 3; i++ {
		resp, err := http.Post(url, "application/json", bytes.NewReader(data))
		if err != nil {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			return nil
		}

		// 503 means leader not ready yet, retry
		if resp.StatusCode == http.StatusServiceUnavailable {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}

		// Other errors, don't retry
		return fmt.Errorf("forward failed: status %d", resp.StatusCode)
	}

	// All retries failed - add to retry buffer for background processing
	s.addToRetryBuffer(entry)
	return nil
}

// addToRetryBuffer adds an entry to the retry buffer and starts the retry worker if needed.
func (s *LogStore) addToRetryBuffer(entry *storage.Entry) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()

	// Limit buffer size
	if len(s.retryBuffer) >= 10000 {
		// Drop oldest entry to make room
		s.retryBuffer = s.retryBuffer[1:]
	}
	s.retryBuffer = append(s.retryBuffer, entry)

	// Start retry worker if not running
	if !s.retryRunning {
		s.retryRunning = true
		s.retryStopChan = make(chan struct{})
		go s.retryWorker()
	}
}

// retryWorker periodically retries forwarding buffered entries to the leader.
func (s *LogStore) retryWorker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.retryStopChan:
			return
		case <-ticker.C:
			s.processRetryBuffer()
		}
	}
}

// processRetryBuffer attempts to forward all buffered entries to the leader.
func (s *LogStore) processRetryBuffer() {
	s.retryMu.Lock()
	if len(s.retryBuffer) == 0 {
		s.retryRunning = false
		if s.retryStopChan != nil {
			close(s.retryStopChan)
			s.retryStopChan = nil
		}
		s.retryMu.Unlock()
		return
	}

	// Get cluster writer
	s.mu.RLock()
	cw := s.clusterWriter
	s.mu.RUnlock()

	if cw == nil {
		s.retryMu.Unlock()
		return
	}

	// Check if we're leader now (can write directly)
	if cw.IsLeader() {
		entries := s.retryBuffer
		s.retryBuffer = nil
		s.retryMu.Unlock()

		for _, entry := range entries {
			s.writeViaRaft(entry)
		}
		return
	}

	// Check if leader is available
	leaderAddr := cw.LeaderAddr()
	if leaderAddr == "" {
		s.retryMu.Unlock()
		return
	}

	// Try to forward entries
	entries := s.retryBuffer
	s.retryBuffer = nil
	s.retryMu.Unlock()

	httpAddr := strings.Replace(leaderAddr, ":4445", ":8080", 1)
	var failed []*storage.Entry

	for _, entry := range entries {
		if err := s.tryForwardEntry(entry, httpAddr); err != nil {
			failed = append(failed, entry)
		}
	}

	// Re-add failed entries
	if len(failed) > 0 {
		s.retryMu.Lock()
		s.retryBuffer = append(failed, s.retryBuffer...)
		s.retryMu.Unlock()
	}
}

// tryForwardEntry attempts to forward a single entry to the leader.
func (s *LogStore) tryForwardEntry(entry *storage.Entry, httpAddr string) error {
	logEntry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     string(entry.GetAttribute("level")[0]),
		Message:   string(entry.GetAttribute("message")[0]),
	}

	if src := entry.GetAttribute("source"); len(src) > 0 {
		logEntry.Source = string(src[0])
	}
	if usr := entry.GetAttribute("user"); len(usr) > 0 {
		logEntry.User = string(usr[0])
	}
	if rid := entry.GetAttribute("requestid"); len(rid) > 0 {
		logEntry.RequestID = string(rid[0])
	}
	if fld := entry.GetAttribute("fields"); len(fld) > 0 {
		json.Unmarshal(fld[0], &logEntry.Fields)
	}

	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/api/v1/internal/log", httpAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("forward failed: status %d", resp.StatusCode)
	}
	return nil
}

// writeLocal writes an entry directly to local database (no Raft).
func (s *LogStore) writeLocal(entry *storage.Entry) error {
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

// trimOldEntries archives and removes oldest entries if we exceed max.
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

	// Get all entries with their data
	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	type entryData struct {
		dn       string
		id       uint64
		logEntry LogEntry
	}
	var entries []entryData

	for iter.Next() {
		entry := iter.Entry()
		if entry.DN == "ou=logs" {
			continue
		}

		ed := entryData{dn: entry.DN}
		if idVals := entry.GetAttribute("logid"); len(idVals) > 0 {
			ed.id, _ = strconv.ParseUint(string(idVals[0]), 10, 64)
		}
		ed.logEntry = s.entryToLogEntry(entry)
		entries = append(entries, ed)
	}
	iter.Close()

	if len(entries) <= s.maxEntries {
		return
	}

	// Sort by ID (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})

	deleteCount := len(entries) - s.maxEntries

	// Archive entries before deleting (if archive is enabled)
	if s.archive != nil && deleteCount > 0 {
		toArchive := make([]LogEntry, deleteCount)
		for i := 0; i < deleteCount; i++ {
			toArchive[i] = entries[i].logEntry
		}
		s.archive.Archive(toArchive)
	}

	// Delete oldest entries
	if deleteCount > 0 {
		// If cluster writer is set, delete through Raft
		if s.clusterWriter != nil {
			for i := 0; i < deleteCount; i++ {
				s.clusterWriter.DeleteLog(entries[i].dn)
			}
			return
		}

		// Standalone mode: direct delete
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

	var allEntries []LogEntry

	// Query active logs from database
	tx, err := s.db.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer s.db.Rollback(tx)

	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	defer iter.Close()

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
		if opts.Source != "" && logEntry.Source != opts.Source {
			continue
		}
		if opts.User != "" && logEntry.User != opts.User {
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
		if opts.Search != "" && !s.matchesSearch(logEntry, opts.Search) {
			continue
		}

		allEntries = append(allEntries, logEntry)
	}

	// Query archived logs if requested
	if opts.IncludeArchive && s.archive != nil {
		archiveOpts := opts
		archiveOpts.Offset = 0 // We'll handle pagination after merging
		archiveOpts.Limit = 0

		var startTime, endTime *time.Time
		if !opts.StartTime.IsZero() {
			startTime = &opts.StartTime
		}
		if !opts.EndTime.IsZero() {
			endTime = &opts.EndTime
		}
		archiveOpts.StartTime = time.Time{}
		archiveOpts.EndTime = time.Time{}

		// Create archive query options
		archiveQueryOpts := QueryOptions{
			Level:     opts.Level,
			Source:    opts.Source,
			User:      opts.User,
			RequestID: opts.RequestID,
			Search:    opts.Search,
		}
		if startTime != nil {
			archiveQueryOpts.StartTime = *startTime
		}
		if endTime != nil {
			archiveQueryOpts.EndTime = *endTime
		}

		archivedEntries, _, _ := s.archive.QueryAllArchives(archiveQueryOpts)
		allEntries = append(allEntries, archivedEntries...)
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
	})

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

// matchesSearch checks if a log entry matches the search term.
// Searches in message, user, and field values.
func (s *LogStore) matchesSearch(entry LogEntry, search string) bool {
	search = strings.ToLower(search)

	// Search in message
	if strings.Contains(strings.ToLower(entry.Message), search) {
		return true
	}

	// Search in user
	if strings.Contains(strings.ToLower(entry.User), search) {
		return true
	}

	// Search in fields
	for _, v := range entry.Fields {
		if str, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(str), search) {
				return true
			}
		}
	}

	return false
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
	if srcVals := entry.GetAttribute("source"); len(srcVals) > 0 {
		logEntry.Source = string(srcVals[0])
	}
	if userVals := entry.GetAttribute("user"); len(userVals) > 0 {
		logEntry.User = string(userVals[0])
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
	Level          string
	Source         string
	User           string
	RequestID      string
	StartTime      time.Time
	EndTime        time.Time
	Search         string
	Offset         int
	Limit          int
	IncludeArchive bool // Include archived logs in search
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

	// If cluster writer is set, delete through Raft
	if s.clusterWriter != nil {
		for _, dn := range dns {
			if err := s.clusterWriter.DeleteLog(dn); err != nil {
				return err
			}
		}
		s.nextID = 1
		return nil
	}

	// Standalone mode: direct delete
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

// Engine returns the underlying storage engine for cluster replication.
func (s *LogStore) Engine() storage.StorageEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

// ListArchives returns all archive files.
func (s *LogStore) ListArchives() ([]ArchiveFile, error) {
	if s.archive == nil {
		return nil, nil
	}
	return s.archive.ListArchives()
}

// GetArchiveStats returns statistics about archives.
func (s *LogStore) GetArchiveStats() *ArchiveStats {
	if s.archive == nil {
		return nil
	}
	stats := s.archive.GetArchiveStats()
	return &stats
}

// ArchiveNow forces immediate archiving of old entries.
func (s *LogStore) ArchiveNow() (*ArchiveFile, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("archiving not enabled")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil, nil
	}

	// Get all entries
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer s.db.Rollback(tx)

	iter := s.db.SearchByDN(tx, "ou=logs", storage.ScopeOneLevel)
	var entries []LogEntry
	var dns []string

	for iter.Next() {
		entry := iter.Entry()
		if entry.DN == "ou=logs" {
			continue
		}
		entries = append(entries, s.entryToLogEntry(entry))
		dns = append(dns, entry.DN)
	}
	iter.Close()

	if len(entries) == 0 {
		return nil, nil
	}

	// Archive all entries
	archiveFile, err := s.archive.Archive(entries)
	if err != nil {
		return nil, err
	}

	// Delete archived entries from active database
	if s.clusterWriter != nil {
		for _, dn := range dns {
			s.clusterWriter.DeleteLog(dn)
		}
	} else {
		tx2, err := s.db.Begin()
		if err != nil {
			return archiveFile, err
		}
		for _, dn := range dns {
			s.db.Delete(tx2, dn)
		}
		s.db.Commit(tx2)
	}

	return archiveFile, nil
}

// CleanupOldArchives removes archives older than retention period.
func (s *LogStore) CleanupOldArchives() (int, error) {
	if s.archive == nil {
		return 0, nil
	}
	return s.archive.CleanupOldArchives()
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
