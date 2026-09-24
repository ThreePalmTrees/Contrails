package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"contrails/agent"

	"github.com/fsnotify/fsnotify"
)

// testLogger is a no-op agent.Logger for tests.
type testLogger struct{}

func (testLogger) Info(string)    {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

// noopChangeHandler satisfies ChangeHandler without doing anything.
type noopChangeHandler struct{}

func (noopChangeHandler) OnOpenCodeDatabaseChanged() {}

func TestIsOpenCodeDBEvent(t *testing.T) {
	cases := []struct {
		name string
		op   fsnotify.Op
		want bool
	}{
		{"opencode.db", fsnotify.Write, true},
		{"opencode.db", fsnotify.Create, true},
		{"opencode.db-wal", fsnotify.Write, true},
		{"opencode.db-shm", fsnotify.Write, true},
		{"opencode-dev.db", fsnotify.Write, true},
		{"opencode-dev.db-wal", fsnotify.Write, true},
		{"opencode-dev.db-shm", fsnotify.Create, true},
		{"opencode.db", fsnotify.Remove, false},
		{"opencode.db-wal", fsnotify.Chmod, false},
		{"state.vscdb", fsnotify.Write, false},
		{"other.db", fsnotify.Write, false},
		{"opencode.jsonc", fsnotify.Write, false},
	}

	for _, tc := range cases {
		event := fsnotify.Event{Name: tc.name, Op: tc.op}
		if got := isOpenCodeDBEvent(event); got != tc.want {
			t.Errorf("isOpenCodeDBEvent(%s, %v) = %v, want %v", tc.name, tc.op, got, tc.want)
		}
	}
}

func TestProcessAll_DedupSkipsUnchangedSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	directory := t.TempDir()
	outputDir := t.TempDir()

	insertScanSession(t, db, "ses_dedup", "", directory, "Dedup session", 1700000000000, 1700000000000, nil)
	insertScanMessage(t, db, "msg_dedup_1", "ses_dedup", "user", 1, 1700000001000, userMessageData("hello"))

	driver := NewDriver(noopChangeHandler{}, testLogger{})
	callbacks := agent.ProcessCallbacks{Filters: agent.DefaultContrailFilters()}

	// First run processes the unseen session and writes one contrail.
	var processedFiles int
	callbacks.OnFileProcessed = func(string, int64) { processedFiles++ }
	count, err := driver.processAll(db, directory, outputDir, callbacks)
	if err != nil {
		t.Fatalf("processAll: %v", err)
	}
	if count != 1 {
		t.Fatalf("first run processed %d sessions, want 1", count)
	}
	if processedFiles != 1 {
		t.Errorf("OnFileProcessed called %d times, want 1", processedFiles)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d files, want 1", len(entries))
	}

	// Second run: nothing changed, nothing processed.
	count, err = driver.processAll(db, directory, outputDir, agent.ProcessCallbacks{Filters: agent.DefaultContrailFilters()})
	if err != nil {
		t.Fatalf("processAll (unchanged): %v", err)
	}
	if count != 0 {
		t.Errorf("unchanged run processed %d sessions, want 0", count)
	}

	// A new message time is a change, and the session is processed again.
	if _, err := db.Exec(
		`UPDATE session_message SET time_updated = ? WHERE id = ?`,
		1700000002000, "msg_dedup_1",
	); err != nil {
		t.Fatal(err)
	}
	count, err = driver.processAll(db, directory, outputDir, agent.ProcessCallbacks{Filters: agent.DefaultContrailFilters()})
	if err != nil {
		t.Fatalf("processAll (bumped): %v", err)
	}
	if count != 1 {
		t.Errorf("bumped run processed %d sessions, want 1", count)
	}
}

func TestProcessAll_SkipsIgnoredSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	directory := t.TempDir()
	outputDir := t.TempDir()

	insertScanSession(t, db, "ses_ignored", "", directory, "Ignored session", 1700000000000, 1700000000000, nil)
	insertScanMessage(t, db, "msg_ignored", "ses_ignored", "user", 1, 1700000001000, userMessageData("hi"))

	driver := NewDriver(noopChangeHandler{}, testLogger{})
	callbacks := agent.ProcessCallbacks{
		Filters:    agent.DefaultContrailFilters(),
		ShouldSkip: func(sessionID string) bool { return sessionID == "ses_ignored" },
	}

	count, err := driver.processAll(db, directory, outputDir, callbacks)
	if err != nil {
		t.Fatalf("processAll: %v", err)
	}
	if count != 0 {
		t.Errorf("processed %d sessions, want 0 (ShouldSkip)", count)
	}
	entries, _ := os.ReadDir(outputDir)
	if len(entries) != 0 {
		t.Errorf("wrote %d files, want 0", len(entries))
	}
}

func TestProcessAll_EmptySourceDir(t *testing.T) {
	driver := NewDriver(noopChangeHandler{}, testLogger{})
	if _, err := driver.ProcessAll("", t.TempDir(), agent.ProcessCallbacks{}); err == nil {
		t.Fatal("expected an error for an empty sourceDir, got nil")
	}
}

func TestDriver_ActivateDeactivateWatcherLifecycle(t *testing.T) {
	// Point the data directory at a temp path so the test never watches the
	// real OpenCode installation.
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}

	driver := NewDriver(noopChangeHandler{}, testLogger{})

	if err := driver.Activate("project-1", "/tmp/opencode-project", ""); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if len(driver.registered) != 1 {
		t.Fatalf("registered = %d, want 1", len(driver.registered))
	}
	if driver.fsWatcher == nil {
		t.Fatal("watcher was not started on first Activate")
	}

	// Registering the same directory again is idempotent.
	if err := driver.Activate("project-1", "/tmp/opencode-project", ""); err != nil {
		t.Fatalf("Activate (repeat): %v", err)
	}
	if len(driver.registered) != 1 {
		t.Fatalf("registered = %d after repeat, want 1", len(driver.registered))
	}

	driver.Deactivate("/tmp/opencode-project", "")
	if len(driver.registered) != 0 {
		t.Fatalf("registered = %d after Deactivate, want 0", len(driver.registered))
	}
	if driver.fsWatcher != nil {
		t.Fatal("watcher was not stopped after the last Deactivate")
	}
}
