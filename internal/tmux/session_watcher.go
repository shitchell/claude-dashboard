package tmux

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/shitchell/claude-dashboard/internal/logging"
)

// SessionWatcher monitors session directories for new .jsonl files using fsnotify.
// When Claude executes /clear, it creates a new session file in the project directory.
// This watcher detects such events and triggers callbacks for cache invalidation.
type SessionWatcher struct {
	mu sync.Mutex

	// watcher is the underlying fsnotify watcher.
	watcher *fsnotify.Watcher

	// dirs tracks which directories are being watched.
	dirs map[string]bool

	// onNewFile is called when a new .jsonl file is created.
	onNewFile func(path string)

	// onError is called when an error occurs during watching.
	onError func(err error)

	// done is used to signal the watch goroutine to stop.
	done chan struct{}

	// running indicates whether the watch loop is active.
	running bool
}

// NewSessionWatcher creates a new SessionWatcher.
// The onNewFile callback is invoked when new .jsonl files are created.
// Call Start() to begin watching, and Stop() to clean up.
func NewSessionWatcher(onNewFile func(path string)) (*SessionWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logging.Warn("SessionWatcher: Failed to create fsnotify watcher: %v", err)
		return nil, err
	}

	sw := &SessionWatcher{
		watcher:   watcher,
		dirs:      make(map[string]bool),
		onNewFile: onNewFile,
		done:      make(chan struct{}),
	}

	logging.Debug("SessionWatcher: Created successfully")
	return sw, nil
}

// SetErrorHandler sets a callback for error handling.
// This is optional and can be used for logging or fallback behavior.
func (sw *SessionWatcher) SetErrorHandler(handler func(err error)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.onError = handler
}

// WatchDirectory adds a directory to be watched for new .jsonl files.
// Returns nil if the directory is already being watched.
func (sw *SessionWatcher) WatchDirectory(dir string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Normalize the path
	dir = filepath.Clean(dir)

	// Check if already watching
	if sw.dirs[dir] {
		logging.Debug("SessionWatcher: Already watching %s", dir)
		return nil
	}

	// Add to fsnotify watcher
	if err := sw.watcher.Add(dir); err != nil {
		logging.Warn("SessionWatcher: Failed to watch %s: %v", dir, err)
		return err
	}

	sw.dirs[dir] = true
	logging.Info("SessionWatcher: Now watching %s", dir)
	return nil
}

// UnwatchDirectory removes a directory from the watch list.
func (sw *SessionWatcher) UnwatchDirectory(dir string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	dir = filepath.Clean(dir)

	if !sw.dirs[dir] {
		return nil
	}

	if err := sw.watcher.Remove(dir); err != nil {
		logging.Warn("SessionWatcher: Failed to unwatch %s: %v", dir, err)
		return err
	}

	delete(sw.dirs, dir)
	logging.Debug("SessionWatcher: Stopped watching %s", dir)
	return nil
}

// WatchedDirs returns a copy of the currently watched directories.
func (sw *SessionWatcher) WatchedDirs() []string {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	dirs := make([]string, 0, len(sw.dirs))
	for dir := range sw.dirs {
		dirs = append(dirs, dir)
	}
	return dirs
}

// Start begins the event processing loop in a goroutine.
// Call Stop() to terminate the loop and clean up resources.
func (sw *SessionWatcher) Start() {
	sw.mu.Lock()
	if sw.running {
		sw.mu.Unlock()
		logging.Debug("SessionWatcher: Already running")
		return
	}
	sw.running = true
	sw.mu.Unlock()

	logging.Info("SessionWatcher: Starting event loop")

	go func() {
		for {
			select {
			case <-sw.done:
				logging.Debug("SessionWatcher: Event loop terminated")
				return

			case event, ok := <-sw.watcher.Events:
				if !ok {
					logging.Debug("SessionWatcher: Events channel closed")
					return
				}
				sw.handleEvent(event)

			case err, ok := <-sw.watcher.Errors:
				if !ok {
					logging.Debug("SessionWatcher: Errors channel closed")
					return
				}
				sw.handleError(err)
			}
		}
	}()
}

// handleEvent processes a single fsnotify event.
func (sw *SessionWatcher) handleEvent(event fsnotify.Event) {
	// We only care about CREATE events for .jsonl files
	if event.Op&fsnotify.Create == 0 {
		return
	}

	// Check if it's a .jsonl file
	if !strings.HasSuffix(event.Name, ".jsonl") {
		return
	}

	// Skip non-UUID session files (like agent-*.jsonl)
	filename := filepath.Base(event.Name)
	sessionID := strings.TrimSuffix(filename, ".jsonl")
	if !IsUUIDSessionFile(sessionID) {
		logging.Debug("SessionWatcher: Ignoring non-UUID file: %s", event.Name)
		return
	}

	logging.Info("SessionWatcher: New session file detected: %s", event.Name)

	// Invoke callback
	sw.mu.Lock()
	callback := sw.onNewFile
	sw.mu.Unlock()

	if callback != nil {
		callback(event.Name)
	}
}

// handleError processes an fsnotify error.
func (sw *SessionWatcher) handleError(err error) {
	logging.Warn("SessionWatcher: fsnotify error: %v", err)

	sw.mu.Lock()
	handler := sw.onError
	sw.mu.Unlock()

	if handler != nil {
		handler(err)
	}
}

// Stop terminates the event loop and releases resources.
func (sw *SessionWatcher) Stop() error {
	sw.mu.Lock()
	if !sw.running {
		sw.mu.Unlock()
		return nil
	}
	sw.running = false
	sw.mu.Unlock()

	logging.Info("SessionWatcher: Stopping")

	// Signal the goroutine to stop
	close(sw.done)

	// Close the underlying watcher
	if err := sw.watcher.Close(); err != nil {
		logging.Warn("SessionWatcher: Error closing watcher: %v", err)
		return err
	}

	logging.Debug("SessionWatcher: Stopped successfully")
	return nil
}

// IsRunning returns whether the watch loop is active.
func (sw *SessionWatcher) IsRunning() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.running
}

// WatchDirectories adds multiple directories to be watched.
// Errors are logged but don't stop watching of other directories.
func (sw *SessionWatcher) WatchDirectories(dirs []string) {
	for _, dir := range dirs {
		if err := sw.WatchDirectory(dir); err != nil {
			logging.Warn("SessionWatcher: Failed to watch %s: %v", dir, err)
		}
	}
}

// SyncWatchedDirs updates the watched directories to match the given set.
// Directories not in the new set are unwatched; new directories are watched.
func (sw *SessionWatcher) SyncWatchedDirs(newDirs []string) {
	sw.mu.Lock()
	currentDirs := make(map[string]bool)
	for dir := range sw.dirs {
		currentDirs[dir] = true
	}
	sw.mu.Unlock()

	// Build set of new directories
	newDirSet := make(map[string]bool)
	for _, dir := range newDirs {
		newDirSet[filepath.Clean(dir)] = true
	}

	// Unwatch directories no longer needed
	for dir := range currentDirs {
		if !newDirSet[dir] {
			sw.UnwatchDirectory(dir)
		}
	}

	// Watch new directories
	for dir := range newDirSet {
		if !currentDirs[dir] {
			sw.WatchDirectory(dir)
		}
	}
}
