package opencode

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// insertScanSession inserts a session_v2 row with an explicit directory and
// optional archive time. It complements insertTestSession, which fixes the
// directory.
func insertScanSession(t *testing.T, db *sql.DB, id, parentID, directory, title string, created, updated int64, archived *int64) {
	t.Helper()

	var parent any
	if parentID != "" {
		parent = parentID
	}
	_, err := db.Exec(`
		INSERT INTO session_v2 (
			id, project_id, parent_id, slug, directory, title, version,
			tokens_input, tokens_output, tokens_reasoning,
			tokens_cache_read, tokens_cache_write, cost,
			time_created, time_updated, time_archived, resume_attempts
		) VALUES (?, 'proj_test', ?, 'test-slug', ?, ?, 'test', 0, 0, 0, 0, 0, 0, ?, ?, ?, 0)`,
		id, parent, directory, title, created, updated, archived,
	)
	if err != nil {
		t.Fatalf("insert scan session %s: %v", id, err)
	}
}

// insertScanMessage inserts a session_message row with an explicit
// time_updated, which the activity queries key on.
func insertScanMessage(t *testing.T, db *sql.DB, id, sessionID, typ string, seq, timeUpdated int64, data string) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO session_message (id, session_id, type, seq, time_created, time_updated, data)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, sessionID, typ, seq, timeUpdated, timeUpdated, data,
	)
	if err != nil {
		t.Fatalf("insert scan message %s: %v", id, err)
	}
}

// userMessageData builds a minimal user message payload.
func userMessageData(text string) string {
	return fmt.Sprintf(`{"time":{"created":1},"text":%q}`, text)
}

// int64Ptr returns a pointer to v, for nullable archive times.
func int64Ptr(v int64) *int64 { return &v }

func findProject(projects []ScannedProject, directory string) (ScannedProject, bool) {
	for _, project := range projects {
		if project.Directory == directory {
			return project, true
		}
	}
	return ScannedProject{}, false
}

func TestBrowseProjects_GroupsByDirectoryAndExcludesChildren(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	projects, err := browseProjects(db)
	if err != nil {
		t.Fatalf("browseProjects: %v", err)
	}

	// The fixture holds two top-level sessions in distinct directories. The
	// child session shares /private/tmp/oc-q2-16007 with its parent but must
	// not be counted, and the errored-shell session is itself a child of a
	// session absent from the fixture.
	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2: %+v", len(projects), projects)
	}

	q2, ok := findProject(projects, "/private/tmp/oc-q2-16007")
	if !ok {
		t.Fatalf("project /private/tmp/oc-q2-16007 not found: %+v", projects)
	}
	if q2.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1 (child must be excluded)", q2.SessionCount)
	}
	if q2.DisplayName != "oc-q2-16007" {
		t.Errorf("DisplayName = %q, want %q", q2.DisplayName, "oc-q2-16007")
	}

	if _, ok := findProject(projects, "/private/tmp/oc-check4/cfg1"); ok {
		t.Error("a directory holding only a subagent session was returned")
	}
}

func TestBrowseProjects_ExcludesArchivedAndEmptyDirectory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	insertScanSession(t, db, "ses_archived", "", "/tmp/oc-archived", "archived", 1, 2, int64Ptr(3))
	insertScanSession(t, db, "ses_empty_dir", "", "", "no directory", 1, 2, nil)
	insertScanSession(t, db, "ses_kept", "", "/tmp/oc-kept", "kept", 1, 2, nil)

	projects, err := browseProjects(db)
	if err != nil {
		t.Fatalf("browseProjects: %v", err)
	}

	if _, ok := findProject(projects, "/tmp/oc-archived"); ok {
		t.Error("archived session's directory was returned")
	}
	if _, ok := findProject(projects, ""); ok {
		t.Error("empty-directory session was returned")
	}
	if _, ok := findProject(projects, "/tmp/oc-kept"); !ok {
		t.Errorf("active session's directory missing: %+v", projects)
	}
}

func TestBrowseProjects_SymlinkNormalisation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	insertScanSession(t, db, "ses_link_a", "", real, "a", 1000, 1000, nil)
	insertScanSession(t, db, "ses_link_b", "", link, "b", 2000, 2000, nil)

	projects, err := browseProjects(db)
	if err != nil {
		t.Fatalf("browseProjects: %v", err)
	}

	normalized := normalizeDirectory(real)
	project, ok := findProject(projects, normalized)
	if !ok {
		t.Fatalf("normalised project %q not found: %+v", normalized, projects)
	}
	if project.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2 (both spellings group together)", project.SessionCount)
	}
	if project.DisplayName != "real" {
		t.Errorf("DisplayName = %q, want %q", project.DisplayName, "real")
	}
}

func TestListSessions_ExcludesChildrenAndArchived(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// The parent is top-level; the child in the same directory is not.
	sessions, err := listSessions(db, "/private/tmp/oc-q2-16007")
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != parentSessionID {
		t.Fatalf("listSessions = %+v, want only %s", sessions, parentSessionID)
	}
	if sessions[0].Title != "parent-probe" {
		t.Errorf("Title = %q, want %q", sessions[0].Title, "parent-probe")
	}

	// This directory's only session is a subagent child.
	children, err := listSessions(db, "/private/tmp/oc-check4/cfg1")
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("listSessions returned %d subagent sessions, want 0", len(children))
	}

	insertScanSession(t, db, "ses_list_archived", "", "/tmp/oc-list", "archived", 1, 2, int64Ptr(3))
	insertScanSession(t, db, "ses_list_kept", "", "/tmp/oc-list", "kept", 4, 5, nil)

	kept, err := listSessions(db, "/tmp/oc-list")
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}
	if len(kept) != 1 || kept[0].ID != "ses_list_kept" {
		t.Fatalf("listSessions = %+v, want only ses_list_kept", kept)
	}
}

func TestListSessions_OrdersByMessageTimeNotSessionRow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	base := t.TempDir()
	insertScanSession(t, db, "ses_old", "", base, "old", 1000, 1000, nil)
	insertScanMessage(t, db, "msg_old", "ses_old", "user", 1, 5000, userMessageData("old"))
	insertScanSession(t, db, "ses_new", "", base, "new", 2000, 2000, nil)
	insertScanMessage(t, db, "msg_new", "ses_new", "user", 1, 9000, userMessageData("new"))

	sessions, err := listSessions(db, base)
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if sessions[0].ID != "ses_new" || sessions[1].ID != "ses_old" {
		t.Fatalf("order = [%s %s], want [ses_new ses_old]", sessions[0].ID, sessions[1].ID)
	}
	// The message time, not the session row's time_updated, is the activity.
	if sessions[0].LastActivityAt != 9000 {
		t.Errorf("LastActivityAt = %d, want 9000 (message time)", sessions[0].LastActivityAt)
	}
	if sessions[1].LastActivityAt != 5000 {
		t.Errorf("LastActivityAt = %d, want 5000 (message time)", sessions[1].LastActivityAt)
	}
}

func TestListSessions_SymlinkTargetMatches(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	insertScanSession(t, db, "ses_symlink", "", real, "symlinked", 1000, 1000, nil)

	for _, query := range []string{real, link} {
		sessions, err := listSessions(db, query)
		if err != nil {
			t.Fatalf("listSessions(%q): %v", query, err)
		}
		if len(sessions) != 1 || sessions[0].ID != "ses_symlink" {
			t.Errorf("listSessions(%q) = %+v, want ses_symlink", query, sessions)
		}
	}
}
