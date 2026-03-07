package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"contrails/agent"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

// ChangeHandler is notified when the Cursor state database changes.
// The application layer implements this interface to bridge driver events
// back into the project processing pipeline.
type ChangeHandler interface {
	// OnCursorDatabaseChanged is called (from a background goroutine) whenever
	// a write is detected on the Cursor state database. The implementation
	// should process all active Cursor projects and is expected to be
	// non-blocking or to manage its own goroutine lifetime.
	OnCursorDatabaseChanged()
}

// Driver implements agent.AgentDriver for Cursor.
// It opens the Cursor SQLite state database read-only, discovers composer
// sessions whose file paths fall under the registered workspace, and writes
// contrail markdown files.
//
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.AgentDriver = (*Driver)(nil)

// Driver is the Cursor agent driver.
type Driver struct {
	// mu protects all fields below.
	mu sync.Mutex
	// registered maps workspacePath → the directory being watched for that
	// workspace's per-workspace state.vscdb. Empty string means no watch dir
	// was resolved (workspace not found in workspaceStorage).
	registered  map[string]string
	watchedDirs map[string]int // dir path → reference count
	fsWatcher   *fsnotify.Watcher
	stopCh      chan struct{}
	doneCh      chan struct{}

	// lastTimestamps tracks the lastUpdatedAt value (Unix ms) we last
	// processed for each composer ID. Used to skip unchanged sessions.
	lastTimestamps map[string]int64

	changeHandler ChangeHandler
	logger        agent.Logger
}

// NewDriver creates a Cursor driver. handler receives database-change
// notifications; logger is used for non-fatal warnings.
func NewDriver(handler ChangeHandler, logger agent.Logger) *Driver {
	return &Driver{
		changeHandler:  handler,
		logger:         logger,
		registered:     make(map[string]string),
		watchedDirs:    make(map[string]int),
		lastTimestamps: make(map[string]int64),
	}
}

// Setup is a no-op for Cursor — no hooks or directories need to be created.
func (d *Driver) Setup(_ string) error { return nil }

// Teardown is a no-op for Cursor — no hooks need to be removed.
func (d *Driver) Teardown(_ string) error { return nil }

// Activate registers a workspace and adds its per-workspace state.vscdb
// directory to the file watcher.
func (d *Driver) Activate(_, workspacePath, _ string) error {
	if workspacePath == "" {
		return nil
	}

	// Resolve the per-workspace watch directory outside the lock (I/O).
	watchDir := resolveWatchDir(workspacePath)

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, already := d.registered[workspacePath]; already {
		return nil
	}
	d.registered[workspacePath] = watchDir

	// Start watcher on first registration.
	if d.fsWatcher == nil {
		if err := d.startWatcherLocked(); err != nil {
			agent.LogWarningf(d.logger, "Cursor database watcher failed to start: %v", err)
			// Non-fatal: ProcessAll still works without real-time watching.
		}
	}

	// Add the per-workspace dir to the running watcher.
	if watchDir != "" && d.fsWatcher != nil {
		d.addWatchDirLocked(watchDir)
	}

	return nil
}

// Deactivate unregisters a workspace and removes its watch directory.
// When no workspaces remain the watcher is stopped entirely.
func (d *Driver) Deactivate(workspacePath, _ string) {
	if workspacePath == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	watchDir, ok := d.registered[workspacePath]
	if !ok {
		return
	}
	delete(d.registered, workspacePath)

	if watchDir != "" && d.fsWatcher != nil {
		d.removeWatchDirLocked(watchDir)
	}

	if len(d.registered) == 0 && d.fsWatcher != nil {
		d.stopWatcherLocked()
	}
}

// ProcessAll enumerates composer sessions for the given workspace,
// parses each one, and writes contrail markdown files to outputDir.
// Sessions whose lastUpdatedAt timestamp has not changed since the
// previous call are skipped to avoid redundant work.
func (d *Driver) ProcessAll(sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) {
	if sourceDir == "" {
		return 0, fmt.Errorf("cursor: sourceDir (workspace path) must not be empty")
	}

	composers, err := ListComposers(sourceDir)
	if err != nil {
		return 0, fmt.Errorf("cursor: listing composers: %w", err)
	}

	total := len(composers)
	if total == 0 {
		return 0, nil
	}

	// Seed lastTimestamps for composers we haven't seen before.
	// Record their current timestamp so we only process future changes,
	// not replay old sessions on every app restart.
	d.mu.Lock()
	for _, c := range composers {
		if _, ok := d.lastTimestamps[c.ID]; !ok {
			d.lastTimestamps[c.ID] = c.LastUpdatedAt
		}
	}
	d.mu.Unlock()

	d.mu.Lock()
	timestamps := d.lastTimestamps
	d.mu.Unlock()

	parser := &Parser{}
	count := 0

	for i, c := range composers {
		composerID := c.ID
		if callbacks.OnProgress != nil {
			callbacks.OnProgress(i+1, total)
		}

		// Skip sessions whose lastUpdatedAt hasn't changed.
		if prev, ok := timestamps[composerID]; ok && c.LastUpdatedAt <= prev {
			continue
		}

		start := time.Now()
		session, err := parser.ParseFile(composerID)
		if err != nil {
			agent.LogWarningf(d.logger, "Cursor: error parsing composer %s: %v", composerID[:8], err)
			continue
		}

		if len(session.Messages) == 0 {
			// Record the timestamp even for empty sessions so we don't
			// re-parse them on every tick.
			d.mu.Lock()
			d.lastTimestamps[composerID] = c.LastUpdatedAt
			d.mu.Unlock()
			continue
		}

		outputPath, err := agent.WriteParsedSession(session, outputDir)
		if err != nil {
			agent.LogWarningf(d.logger, "Cursor: error writing contrail for %s: %v", composerID[:8], err)
			continue
		}

		// Record the timestamp after successful write.
		d.mu.Lock()
		d.lastTimestamps[composerID] = c.LastUpdatedAt
		d.mu.Unlock()

		count++
		if callbacks.OnFileProcessed != nil {
			callbacks.OnFileProcessed(filepath.Base(outputPath), time.Since(start).Milliseconds())
		}
	}

	return count, nil
}

// --- Watcher internals ---

// resolveWatchDir finds the workspaceStorage subdirectory for the given
// workspacePath and returns the directory containing its state.vscdb.
// Returns "" if the workspace is not found.
func resolveWatchDir(workspacePath string) string {
	entries, err := scanWorkspaceStorage()
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.workspacePath == workspacePath {
			return filepath.Dir(entry.dbPath)
		}
	}
	return ""
}

// addWatchDirLocked adds dir to the fsnotify watcher, tracking a reference
// count so that shared directories are only removed when the last user goes.
// Must be called with d.mu held.
func (d *Driver) addWatchDirLocked(dir string) {
	d.watchedDirs[dir]++
	if d.watchedDirs[dir] == 1 {
		if err := d.fsWatcher.Add(dir); err != nil {
			agent.LogWarningf(d.logger, "Cursor: failed to watch %s: %v", dir, err)
		}
	}
}

// removeWatchDirLocked decrements the reference count for dir and removes it
// from the watcher when the count reaches zero.
// Must be called with d.mu held.
func (d *Driver) removeWatchDirLocked(dir string) {
	d.watchedDirs[dir]--
	if d.watchedDirs[dir] <= 0 {
		delete(d.watchedDirs, dir)
		_ = d.fsWatcher.Remove(dir)
	}
}

// globalStorageDir returns the directory containing Cursor's global state.vscdb.
func globalStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage")
}

// startWatcherLocked starts the fsnotify watcher and its event loop.
// Must be called with d.mu held.
func (d *Driver) startWatcherLocked() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	d.fsWatcher = w
	d.stopCh = make(chan struct{})
	d.doneCh = make(chan struct{})

	// Always watch the global state.vscdb — this is where bubble (message)
	// content is written on every Cursor interaction, regardless of workspace.
	if dir := globalStorageDir(); dir != "" {
		if err := w.Add(dir); err != nil {
			agent.LogWarningf(d.logger, "Cursor: failed to watch global storage dir: %v", err)
		}
	}

	// Debounce: Cursor writes the WAL and/or main DB file in rapid bursts.
	// Wait 2 s of inactivity before notifying the handler to avoid
	// hammering the database while Cursor is actively writing.
	debounced := debounce.New(2 * time.Second)

	go func() {
		defer close(d.doneCh)
		for {
			select {
			case <-d.stopCh:
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if isCursorDBEvent(event) {
					debounced(d.changeHandler.OnCursorDatabaseChanged)
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				agent.LogWarningf(d.logger, "Cursor watcher error: %v", err)
			}
		}
	}()

	return nil
}

// stopWatcherLocked stops the database file watcher and waits for the event
// loop goroutine to exit. Must be called with d.mu held.
func (d *Driver) stopWatcherLocked() {
	if d.fsWatcher == nil {
		return
	}

	close(d.stopCh)
	d.fsWatcher.Close()

	// Release the lock while waiting so the event goroutine can proceed.
	d.mu.Unlock()
	<-d.doneCh
	d.mu.Lock()

	d.fsWatcher = nil
	d.stopCh = nil
	d.doneCh = nil
	d.watchedDirs = make(map[string]int)
}

// isCursorDBEvent reports whether an fsnotify event is a write to a
// state.vscdb file or its WAL companion.
func isCursorDBEvent(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return false
	}
	name := filepath.Base(event.Name)
	return strings.HasSuffix(name, "state.vscdb") ||
		strings.HasSuffix(name, "state.vscdb-wal")
}
