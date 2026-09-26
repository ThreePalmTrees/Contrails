package opencode

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
)

// ScannedProject represents one working directory that has OpenCode sessions.
type ScannedProject struct {
	// Directory is the absolute, symlink-normalised working directory.
	Directory string `json:"directory"`

	// DisplayName is the last path component of Directory.
	DisplayName string `json:"displayName"`

	// SessionCount is the number of top-level sessions in the directory.
	SessionCount int `json:"sessionCount"`

	// LastActivityAt is the newest message time across the directory's
	// sessions (Unix ms), falling back to the session row's time_updated.
	LastActivityAt int64 `json:"lastActivityAt"`
}

// SessionInfo holds summary metadata for a single OpenCode session.
type SessionInfo struct {
	ID    string
	Title string
	// CreatedAt is the session row's time_created (Unix ms).
	CreatedAt int64
	// LastActivityAt is the newest message time_updated (Unix ms). The
	// session row's own time_updated does not move on message writes, so it
	// is only the fallback when a session has no messages.
	LastActivityAt int64
}

// sessionSummary is one top-level session row reduced to the fields the
// scanner and driver need.
type sessionSummary struct {
	id        string
	title     string
	directory string
	createdAt int64
	activity  int64
}

// BrowseProjects returns one entry per distinct working directory that has
// top-level OpenCode sessions, sorted newest-first.
func BrowseProjects() ([]ScannedProject, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return browseProjects(db)
}

// browseProjects is the testable core of BrowseProjects: it accepts an
// already-open *sql.DB so tests can supply an in-memory database.
func browseProjects(db *sql.DB) ([]ScannedProject, error) {
	summaries, err := fetchSessionSummaries(db)
	if err != nil {
		return nil, err
	}

	// Group by the normalised directory: two spellings of the same path
	// (e.g. /tmp and /private/tmp on macOS) are one project.
	merged := make(map[string]*ScannedProject)
	for _, summary := range summaries {
		directory := normalizeDirectory(summary.directory)
		if existing, ok := merged[directory]; ok {
			existing.SessionCount++
			if summary.activity > existing.LastActivityAt {
				existing.LastActivityAt = summary.activity
			}
			continue
		}
		merged[directory] = &ScannedProject{
			Directory:      directory,
			DisplayName:    filepath.Base(directory),
			SessionCount:   1,
			LastActivityAt: summary.activity,
		}
	}

	results := make([]ScannedProject, 0, len(merged))
	for _, project := range merged {
		results = append(results, *project)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].LastActivityAt != results[j].LastActivityAt {
			return results[i].LastActivityAt > results[j].LastActivityAt
		}
		return results[i].Directory < results[j].Directory
	})

	return results, nil
}

// ListSessions returns metadata for all top-level OpenCode sessions whose
// working directory matches directory, newest-first. The match is made on
// the symlink-normalised path so a session recorded as /tmp/... is found
// when the project is registered as /private/tmp/....
func ListSessions(directory string) ([]SessionInfo, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return listSessions(db, directory)
}

// listSessions is the testable core of ListSessions: it accepts an
// already-open *sql.DB so tests can supply an in-memory database.
func listSessions(db *sql.DB, directory string) ([]SessionInfo, error) {
	summaries, err := fetchSessionSummaries(db)
	if err != nil {
		return nil, err
	}

	// SQLite cannot normalise symlinks, so the directory filter runs in Go.
	// The database holds only the live sessions (tens to hundreds of rows),
	// so the full scan is cheap.
	target := normalizeDirectory(directory)
	var results []SessionInfo
	for _, summary := range summaries {
		if normalizeDirectory(summary.directory) != target {
			continue
		}
		results = append(results, SessionInfo{
			ID:             summary.id,
			Title:          summary.title,
			CreatedAt:      summary.createdAt,
			LastActivityAt: summary.activity,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].LastActivityAt != results[j].LastActivityAt {
			return results[i].LastActivityAt > results[j].LastActivityAt
		}
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// fetchSessionSummaries returns every top-level, unarchived session with a
// non-empty directory and its activity time.
//
// parent_id IS NULL excludes subagent sessions, which share their parent's
// directory and would otherwise be counted and parsed twice. The activity
// time is the per-session MAX(session_message.time_updated): the session
// row's time_updated does not move on message writes, so it is only the
// fallback for a session with no messages.
func fetchSessionSummaries(db *sql.DB) ([]sessionSummary, error) {
	table, err := sessionTable(db)
	if err != nil {
		return nil, err
	}

	query := `SELECT s.id, s.title, s.directory, s.time_created,
		COALESCE((
			SELECT MAX(m.time_updated) FROM session_message m
			WHERE m.session_id = s.id
		), s.time_updated)
		FROM ` + table + ` s
		WHERE s.time_archived IS NULL AND s.parent_id IS NULL AND s.directory <> ''
		ORDER BY s.time_created ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	var summaries []sessionSummary
	for rows.Next() {
		var (
			summary sessionSummary
			title   sql.NullString
		)
		if err := rows.Scan(&summary.id, &title, &summary.directory, &summary.createdAt, &summary.activity); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		summary.title = title.String
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sessions: %w", err)
	}
	return summaries, nil
}

// normalizeDirectory resolves symlinks so two spellings of one directory
// group together. Paths that no longer exist (a deleted project) fall back
// to their cleaned form.
func normalizeDirectory(directory string) string {
	if directory == "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return filepath.Clean(directory)
	}
	return resolved
}
