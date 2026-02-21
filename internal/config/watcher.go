package config

import (
	"os"
	"sync"
	"time"
)

// ConfigWatcher watches a config file for changes and triggers reload.
type ConfigWatcher struct {
	filePath     string
	pollInterval time.Duration
	debounce     time.Duration
	lastModTime  time.Time
	lastSize     int64
	lastConfig   *Config
	onChange     func(oldCfg, newCfg *Config)
	stopCh       chan struct{}
	stoppedCh    chan struct{}
	mu           sync.Mutex
	running      bool
}

// WatcherConfig holds config watcher configuration.
type WatcherConfig struct {
	FilePath     string
	PollInterval time.Duration // Default: 100ms
	Debounce     time.Duration // Default: 200ms
	OnChange     func(oldCfg, newCfg *Config)
}

// NewConfigWatcher creates a new config file watcher.
func NewConfigWatcher(cfg *WatcherConfig) (*ConfigWatcher, error) {
	if cfg.FilePath == "" {
		return nil, ErrMissingConfigFile
	}
	if cfg.OnChange == nil {
		return nil, ErrMissingOnChange
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

	// Load initial config
	initialConfig, err := LoadConfig(cfg.FilePath)
	if err != nil {
		return nil, err
	}

	return &ConfigWatcher{
		filePath:     cfg.FilePath,
		pollInterval: pollInterval,
		debounce:     debounce,
		lastModTime:  info.ModTime(),
		lastSize:     info.Size(),
		lastConfig:   initialConfig,
		onChange:     cfg.OnChange,
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
	}, nil
}

// Start begins watching the config file for changes.
func (w *ConfigWatcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.watchLoop()
}

// Stop stops watching the config file.
func (w *ConfigWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)
	<-w.stoppedCh
}

// watchLoop is the main polling loop.
func (w *ConfigWatcher) watchLoop() {
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
				continue
			}

			if changed {
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

// checkFileChanged checks if the config file has been modified.
func (w *ConfigWatcher) checkFileChanged() (bool, error) {
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

// triggerReload loads the new config and calls onChange.
func (w *ConfigWatcher) triggerReload() {
	newConfig, err := LoadConfig(w.filePath)
	if err != nil {
		return
	}

	// Validate new config
	errs := ValidateConfig(newConfig)
	if len(errs) > 0 {
		return
	}

	oldConfig := w.lastConfig
	w.lastConfig = newConfig

	w.onChange(oldConfig, newConfig)
}

// IsRunning returns true if the watcher is running.
func (w *ConfigWatcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// GetCurrentConfig returns the last loaded config.
func (w *ConfigWatcher) GetCurrentConfig() *Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastConfig
}
