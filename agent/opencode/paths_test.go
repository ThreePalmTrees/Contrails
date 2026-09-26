package opencode

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setDataHome points dataDir at a fresh temporary directory, clears
// OPENCODE_DB, and returns the resolved OpenCode data directory (created).
func setDataHome(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("OPENCODE_DB", "")
	dir := filepath.Join(tmp, "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeFileAt creates path with placeholder content and the given
// modification time.
func writeFileAt(t *testing.T, path string, mod time.Time) {
	t.Helper()

	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestDataDir_XDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if want := filepath.Join(tmp, "opencode"); got != want {
		t.Errorf("dataDir = %q, want %q", got, want)
	}
}

func TestDataDir_DefaultsToLocalShare(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows

	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if want := filepath.Join(home, ".local", "share", "opencode"); got != want {
		t.Errorf("dataDir = %q, want %q", got, want)
	}
}

func TestDBPath_OpenCodeDBOverride(t *testing.T) {
	dir := setDataHome(t)
	// A stable database must not win over an explicit override.
	writeFileAt(t, filepath.Join(dir, "opencode.db"), time.Now())

	absolute := filepath.Join(t.TempDir(), "opencode-alt.db")
	t.Setenv("OPENCODE_DB", absolute)
	if got, err := dbPath(); err != nil || got != absolute {
		t.Errorf("absolute override: got (%q, %v), want %q", got, err, absolute)
	}

	t.Setenv("OPENCODE_DB", ":memory:")
	if got, err := dbPath(); err != nil || got != ":memory:" {
		t.Errorf(":memory: override: got (%q, %v)", got, err)
	}

	t.Setenv("OPENCODE_DB", "relative.db")
	want := filepath.Join(dir, "relative.db")
	if got, err := dbPath(); err != nil || got != want {
		t.Errorf("relative override: got (%q, %v), want %q", got, err, want)
	}
}

// A channel database that is newer on disk (e.g. a dev build run once) must
// not replace the stable database: in WAL mode the stable file's date only
// moves on checkpoint, so dates do not tell which database is in use.
func TestDBPath_StablePreferredOverNewerChannel(t *testing.T) {
	dir := setDataHome(t)
	stable := filepath.Join(dir, "opencode.db")
	writeFileAt(t, stable, time.Now().Add(-time.Hour))
	writeFileAt(t, filepath.Join(dir, "opencode-dev.db"), time.Now())

	got, err := dbPath()
	if err != nil {
		t.Fatalf("dbPath: %v", err)
	}
	if got != stable {
		t.Errorf("dbPath = %q, want the stable database %q", got, stable)
	}
}

func TestDBPath_NewestChannelWhenNoStable(t *testing.T) {
	dir := setDataHome(t)
	older := filepath.Join(dir, "opencode-beta.db")
	newer := filepath.Join(dir, "opencode-dev.db")
	writeFileAt(t, older, time.Now().Add(-time.Hour))
	writeFileAt(t, newer, time.Now())
	// Sidecars and directories are not databases.
	writeFileAt(t, filepath.Join(dir, "opencode-beta.db-wal"), time.Now().Add(time.Hour))
	if err := os.Mkdir(filepath.Join(dir, "opencode-dir.db"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := dbPath()
	if err != nil {
		t.Fatalf("dbPath: %v", err)
	}
	if got != newer {
		t.Errorf("dbPath = %q, want the newest channel database %q", got, newer)
	}
}

func TestDBPath_NoDatabaseFallsBackToStablePath(t *testing.T) {
	dir := setDataHome(t)

	got, err := dbPath()
	if err != nil {
		t.Fatalf("dbPath: %v", err)
	}
	if want := filepath.Join(dir, "opencode.db"); got != want {
		t.Errorf("dbPath = %q, want %q", got, want)
	}
}
