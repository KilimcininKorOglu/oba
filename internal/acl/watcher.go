// Package acl provides access control list functionality.
package acl

import (
	"os"
	"sync"
	"time"

	"github.com/oba-ldap/oba/internal/logging"
)

// FileWatcher watches an ACL file for changes and triggers reload.
type FileWatcher struct {
	filePath     string
	manager      *Manager
	logger       logging.Logger
	pollInterval time.Duration
	debounce     time.Duration
	lastModTime  time.Time
	lastSize     int64
	stopCh       chan struct{}
	stoppedCh    chan struct{}
	mu           sync.Mutex
	running      bool
}

// WatcherConfig holds file watcher configuration.
type WatcherConfig struct {
	FilePath     string
	Manager      *Manager
	Logger       logging.Logger
	PollInterval time.Duration // Default: 100ms
	Debounce     time.Duration // Default: 200ms
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(cfg *WatcherConfig) (*FileWatcher, error) {
	if cfg.FilePath == "" {
		return nil, ErrNoFilePath
	}
	if cfg.Manager == nil {
		return nil, ErrInvalidConfig
	}

	pollInterval := cfg.PollInterval
	if pollInterval == 0 {
		pollInterval = 100 * time.Millisecond
	}

	debounce := cfg.Debounce
	if debounce == 0 {
		debounce = 200 * time.Millisecond
	}

	// Get initial file stats
	info, err := os.Stat(cfg.FilePath)
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		filePath:     cfg.FilePath,
		manager:      cfg.Manager,
		logger:       cfg.Logger,
		pollInterval: pollInterval,
		debounce:     debounce,
		lastModTime:  info.ModTime(),
		lastSize:     info.Size(),
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
	}, nil
}

// Start begins watching the file for changes.
func (w *FileWatcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.watchLoop()

	if w.logger != nil {
		w.logger.Info("ACL file watcher started",
			"file", w.filePath,
			"pollInterval", w.pollInterval.String(),
		)
	}
}

// Stop stops watching the file.
func (w *FileWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	<-w.stoppedCh

	if w.logger != nil {
		w.logger.Info("ACL file watcher stopped", "file", w.filePath)
	}
}

// watchLoop is the main polling loop.
func (w *FileWatcher) watchLoop() {
	defer close(w.stoppedCh)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	var pendingReload bool
	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case <-ticker.C:
			changed, err := w.checkFileChanged()
			if err != nil {
				if w.logger != nil {
					w.logger.Error("failed to check ACL file", "error", err)
				}
				continue
			}

			if changed {
				// File changed, start/reset debounce timer
				pendingReload = true
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.NewTimer(w.debounce)
				debounceCh = debounceTimer.C
			}

		case <-debounceCh:
			if pendingReload {
				w.triggerReload()
				pendingReload = false
			}
			debounceTimer = nil
			debounceCh = nil
		}
	}
}

// checkFileChanged checks if the file has been modified.
func (w *FileWatcher) checkFileChanged() (bool, error) {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return false, err
	}

	modTime := info.ModTime()
	size := info.Size()

	if modTime != w.lastModTime || size != w.lastSize {
		w.lastModTime = modTime
		w.lastSize = size
		return true, nil
	}

	return false, nil
}

// triggerReload triggers an ACL reload.
func (w *FileWatcher) triggerReload() {
	if w.logger != nil {
		w.logger.Info("ACL file changed, reloading", "file", w.filePath)
	}

	if err := w.manager.Reload(); err != nil {
		if w.logger != nil {
			w.logger.Error("ACL reload failed", "error", err)
		}
		return
	}

	stats := w.manager.Stats()
	if w.logger != nil {
		w.logger.Info("ACL reloaded successfully",
			"rules", stats.RuleCount,
			"reloadCount", stats.ReloadCount,
		)
	}
}

// IsRunning returns true if the watcher is running.
func (w *FileWatcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}
