package opencode

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"contrails/agent"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

// ChangeHandler is notified when the OpenCode database changes. The
// application layer implements this interface to bridge driver events back
// into the project processing pipeline.
type ChangeHandler interface {
	// OnOpenCodeDatabaseChanged is called (from a background goroutine)
	// whenever a write is detected on the OpenCode database. The
	// implementation should process all active OpenCode projects and is
	// expected to be non-blocking or to manage its own goroutine lifetime.
	OnOpenCodeDatabaseChanged()
}

// Driver implements agent.AgentDriver for OpenCode.
//
// The OpenCode database is global — one file holds every project's sessions
// — so a single fsnotify watcher on the data directory serves all registered
// projects, and Activate/Deactivate only toggle a directory filter.
//
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.AgentDriver = (*Driver)(nil)

// Driver is the OpenCode agent driver.
type Driver struct {
	// mu protects all fields below.
	mu sync.Mutex
	// registered is the set of project directories with an active source.
	// The global database is watched once, regardless of how many are
	// registered.
	registered map[string]struct{}
	fsWatcher  *fsnotify.Watcher
	stopCh     chan struct{}
	doneCh     chan struct{}

	// lastTimestamps tracks the lastActivityAt value (Unix ms) processed
	// for each session ID. Used to skip unchanged sessions.
	lastTimestamps map[string]int64

	changeHandler ChangeHandler
	logger        agent.Logger
}

// NewDriver creates an OpenCode driver. handler receives database-change
// notifications; logger is used for non-fatal warnings.
func NewDriver(handler ChangeHandler, logger agent.Logger) *Driver {
	return &Driver{
		changeHandler:  handler,
		logger:         logger,
		registered:     make(map[string]struct{}),
		lastTimestamps: make(map[string]int64),
	}
}

// Setup is a no-op for OpenCode — it has no hook system.
func (d *Driver) Setup(_ string) error { return nil }

// Teardown is a no-op for OpenCode — it has no hook system.
func (d *Driver) Teardown(_ string) error { return nil }

// Activate registers a project directory and starts the database watcher on
// the first registration. The watcher is global, so workspacePath only
// selects which directory this project reads; it is never watched itself.
func (d *Driver) Activate(_, workspacePath, _ string) error {
	if workspacePath == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, already := d.registered[workspacePath]; already {
		return nil
	}
	d.registered[workspacePath] = struct{}{}

	if d.fsWatcher == nil {
		if err := d.startWatcherLocked(); err != nil {
			agent.LogWarningf(d.logger, "OpenCode database watcher failed to start: %v", err)
			// Non-fatal: ProcessAll still works without real-time watching.
		}
	}

	return nil
}

// Deactivate unregisters a project directory. When no projects remain the
// watcher is stopped entirely.
func (d *Driver) Deactivate(workspacePath, _ string) {
	if workspacePath == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.registered[workspacePath]; !ok {
		return
	}
	delete(d.registered, workspacePath)

	if len(d.registered) == 0 && d.fsWatcher != nil {
		d.stopWatcherLocked()
	}
}

// ProcessAll enumerates sessions for the given directory, parses each one,
// and writes contrail markdown files to outputDir. Sessions whose latest
// message time has not changed since the previous call are skipped.
func (d *Driver) ProcessAll(sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) {
	if sourceDir == "" {
		return 0, fmt.Errorf("opencode: sourceDir (directory) must not be empty")
	}

	db, err := openDB()
	if err != nil {
		return 0, fmt.Errorf("opencode: opening database: %w", err)
	}
	defer db.Close()

	return d.processAll(db, sourceDir, outputDir, callbacks)
}

// processAll is the testable core of ProcessAll: it accepts an already-open
// *sql.DB so tests can supply an in-memory database.
func (d *Driver) processAll(db *sql.DB, sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) {
	sessions, err := listSessions(db, sourceDir)
	if err != nil {
		return 0, fmt.Errorf("opencode: listing sessions: %w", err)
	}

	total := len(sessions)
	if total == 0 {
		return 0, nil
	}

	count := 0

	for i, info := range sessions {
		if callbacks.OnProgress != nil {
			callbacks.OnProgress(i+1, total)
		}

		// Skip ignored chats (the session ID is the "file path" here).
		if callbacks.ShouldSkip != nil && callbacks.ShouldSkip(info.ID) {
			continue
		}

		// Skip sessions whose latest message time hasn't changed.
		d.mu.Lock()
		prev, seen := d.lastTimestamps[info.ID]
		d.mu.Unlock()
		if seen && info.LastActivityAt <= prev {
			continue
		}

		start := time.Now()
		// parseSession is the Parser's testable core; using it directly
		// keeps this loop on one connection (ParseFile would reopen the
		// database for every session).
		session, err := parseSession(db, info.ID)
		if err != nil {
			agent.LogWarningf(d.logger, "OpenCode: error parsing session %s: %v", info.ID, err)
			continue
		}

		if len(session.Messages) == 0 {
			// Record the timestamp even for empty sessions so we don't
			// re-parse them on every tick.
			d.recordTimestamp(info.ID, info.LastActivityAt)
			continue
		}

		agent.ApplyContrailFilters(session, callbacks.Filters)

		outputPath, err := agent.WriteParsedSession(session, outputDir)
		if err != nil {
			agent.LogWarningf(d.logger, "OpenCode: error writing contrail for %s: %v", info.ID, err)
			continue
		}

		// Record the timestamp after successful write.
		d.recordTimestamp(info.ID, info.LastActivityAt)

		count++
		if callbacks.OnFileProcessed != nil {
			callbacks.OnFileProcessed(filepath.Base(outputPath), time.Since(start).Milliseconds())
		}
	}

	return count, nil
}

// recordTimestamp stores the last activity time processed for a session.
func (d *Driver) recordTimestamp(sessionID string, timestamp int64) {
	d.mu.Lock()
	d.lastTimestamps[sessionID] = timestamp
	d.mu.Unlock()
}

// startWatcherLocked starts the fsnotify watcher on the OpenCode data
// directory and its event loop. Must be called with d.mu held.
func (d *Driver) startWatcherLocked() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	d.fsWatcher = w
	d.stopCh = make(chan struct{})
	d.doneCh = make(chan struct{})

	// One global database serves every project, so a single watch on the
	// data directory covers the stable opencode.db and every
	// channel-suffixed database with its WAL sidecars.
	if dir, err := dataDir(); err != nil {
		agent.LogWarningf(d.logger, "OpenCode: cannot resolve data directory: %v", err)
	} else if err := w.Add(dir); err != nil {
		agent.LogWarningf(d.logger, "OpenCode: failed to watch data directory %s: %v", dir, err)
	}

	// Debounce: SQLite writes the main file and the WAL in rapid bursts.
	// Wait 2 s of inactivity before notifying the handler to avoid hammering
	// the database while OpenCode is actively writing.
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
				if isOpenCodeDBEvent(event) {
					debounced(d.changeHandler.OnOpenCodeDatabaseChanged)
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				agent.LogWarningf(d.logger, "OpenCode watcher error: %v", err)
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
}

// isOpenCodeDBEvent reports whether an fsnotify event is a write to an
// OpenCode database file or one of its WAL sidecars. Channel-suffixed
// builds write opencode-<channel>.db{,-wal,-shm}, so matching only
// opencode.db would starve on those channels.
func isOpenCodeDBEvent(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return false
	}
	name := filepath.Base(event.Name)
	for _, suffix := range []string{".db", ".db-wal", ".db-shm"} {
		if strings.HasPrefix(name, "opencode") && strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
