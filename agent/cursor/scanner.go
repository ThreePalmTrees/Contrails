package cursor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScannedProject represents a Cursor workspace discovered during database scanning.
type ScannedProject struct {
	// WorkspacePath is the absolute filesystem path of the project root,
	// or the path to a .code-workspace file for multi-root workspaces.
	WorkspacePath string `json:"workspacePath"`

	// DisplayName is the last path component of WorkspacePath.
	DisplayName string `json:"displayName"`

	// ComposerCount is the number of composer sessions found for this workspace.
	ComposerCount int `json:"composerCount"`

	// LastActivityAt is the most recent lastUpdatedAt across all composers (Unix ms).
	LastActivityAt int64 `json:"lastActivityAt"`
}

// workspaceStorageDir returns the path to Cursor's workspaceStorage directory.
func workspaceStorageDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(
		home, "Library", "Application Support",
		"Cursor", "User", "workspaceStorage",
	), nil
}

// workspaceEntry holds the resolved info for one workspaceStorage subdirectory.
type workspaceEntry struct {
	workspacePath string // folder path or .code-workspace file path
	displayName   string // last path component
	dbPath        string // path to this workspace's state.vscdb
}

// scanWorkspaceStorage reads the workspaceStorage directory and returns one
// entry per subdirectory that has a recognisable workspace.json.
func scanWorkspaceStorage() ([]workspaceEntry, error) {
	storageDir, err := workspaceStorageDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil, fmt.Errorf("reading workspaceStorage: %w", err)
	}

	var results []workspaceEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(storageDir, entry.Name())

		wsPath, displayName := resolveWorkspaceJSON(filepath.Join(dirPath, "workspace.json"))
		if wsPath == "" {
			continue
		}

		dbFile := filepath.Join(dirPath, "state.vscdb")
		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			continue
		}

		results = append(results, workspaceEntry{
			workspacePath: wsPath,
			displayName:   displayName,
			dbPath:        dbFile,
		})
	}
	return results, nil
}

// resolveWorkspaceJSON reads a workspace.json file and extracts the workspace
// path and display name. Returns ("", "") if the file is missing or unreadable.
func resolveWorkspaceJSON(jsonPath string) (workspacePath, displayName string) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", ""
	}
	var cfg struct {
		Folder    string `json:"folder"`
		Workspace string `json:"workspace"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", ""
	}
	var raw string
	if cfg.Folder != "" {
		raw = strings.TrimPrefix(cfg.Folder, "file://")
	} else if cfg.Workspace != "" {
		raw = strings.TrimPrefix(cfg.Workspace, "file://")
	}
	if raw == "" {
		return "", ""
	}
	return raw, filepath.Base(raw)
}

// composerSummary is a minimal decode of the allComposers array stored in
// the per-workspace ItemTable under key "composer.composerData".
type composerSummary struct {
	ComposerID    string `json:"composerId"`
	Name          string `json:"name"`
	CreatedAt     int64  `json:"createdAt"`
	LastUpdatedAt int64  `json:"lastUpdatedAt"`
}

// readWorkspaceComposers opens the per-workspace state.vscdb and returns
// the list of composers from the ItemTable.
func readWorkspaceComposers(dbPath string) ([]composerSummary, error) {
	dsn := "file:" + dbPath + "?mode=ro&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening workspace db: %w", err)
	}
	defer db.Close()

	var raw string
	err = db.QueryRow(
		"SELECT value FROM ItemTable WHERE key = 'composer.composerData'",
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying composer.composerData: %w", err)
	}

	var payload struct {
		AllComposers []composerSummary `json:"allComposers"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("decoding composer.composerData: %w", err)
	}
	return payload.AllComposers, nil
}

// BrowseProjects scans Cursor's workspaceStorage and returns one ScannedProject
// per workspace that has at least one composer session.
// Results are sorted by most-recent activity first.
func BrowseProjects() ([]ScannedProject, error) {
	entries, err := scanWorkspaceStorage()
	if err != nil {
		return nil, fmt.Errorf("scanning workspace storage: %w", err)
	}

	// Multiple workspaceStorage directories can resolve to the same workspace
	// path (e.g. after Cursor updates). Merge them by summing composer counts
	// and keeping the most recent activity timestamp.
	merged := make(map[string]*ScannedProject)
	for _, entry := range entries {
		composers, err := readWorkspaceComposers(entry.dbPath)
		if err != nil || len(composers) == 0 {
			continue
		}

		var lastActivity int64
		for _, c := range composers {
			if c.LastUpdatedAt > lastActivity {
				lastActivity = c.LastUpdatedAt
			}
		}

		if existing, ok := merged[entry.workspacePath]; ok {
			existing.ComposerCount += len(composers)
			if lastActivity > existing.LastActivityAt {
				existing.LastActivityAt = lastActivity
			}
		} else {
			merged[entry.workspacePath] = &ScannedProject{
				WorkspacePath:  entry.workspacePath,
				DisplayName:    entry.displayName,
				ComposerCount:  len(composers),
				LastActivityAt: lastActivity,
			}
		}
	}

	results := make([]ScannedProject, 0, len(merged))
	for _, sp := range merged {
		results = append(results, *sp)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].LastActivityAt != results[j].LastActivityAt {
			return results[i].LastActivityAt > results[j].LastActivityAt
		}
		return results[i].WorkspacePath < results[j].WorkspacePath
	})

	return results, nil
}

// ComposerInfo holds summary metadata for a single Cursor composer session.
type ComposerInfo struct {
	ID            string
	Name          string
	CreatedAt     int64
	LastUpdatedAt int64
}

// ListComposers returns metadata for all composer sessions belonging to the
// workspace identified by workspacePath. Results are sorted newest-first.
func ListComposers(workspacePath string) ([]ComposerInfo, error) {
	entries, err := scanWorkspaceStorage()
	if err != nil {
		return nil, fmt.Errorf("scanning workspace storage: %w", err)
	}

	// Multiple workspaceStorage directories can resolve to the same workspace
	// path. Collect composers from all matching entries.
	seen := make(map[string]struct{})
	var results []ComposerInfo
	for _, entry := range entries {
		if entry.workspacePath != workspacePath {
			continue
		}
		composers, err := readWorkspaceComposers(entry.dbPath)
		if err != nil {
			return nil, err
		}
		for _, c := range composers {
			if c.ComposerID == "" {
				continue
			}
			if _, dup := seen[c.ComposerID]; dup {
				continue
			}
			seen[c.ComposerID] = struct{}{}
			results = append(results, ComposerInfo{
				ID:            c.ComposerID,
				Name:          c.Name,
				CreatedAt:     c.CreatedAt,
				LastUpdatedAt: c.LastUpdatedAt,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastUpdatedAt > results[j].LastUpdatedAt
	})
	return results, nil
}

// commonAncestor returns the longest common directory ancestor of the given
// absolute filesystem paths. For a single path it returns its parent directory.
// Kept for use in tests.
func commonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return filepath.Dir(paths[0])
	}

	split := make([][]string, len(paths))
	for i, p := range paths {
		split[i] = strings.Split(filepath.Clean(p), "/")
	}

	common := split[0]
	for _, parts := range split[1:] {
		n := min(len(common), len(parts))
		j := 0
		for j < n && common[j] == parts[j] {
			j++
		}
		common = common[:j]
	}

	if len(common) == 0 {
		return "/"
	}
	return strings.Join(common, "/")
}

// stripFileScheme removes the "file://" scheme prefix from a URI.
// Kept for use in tests.
func stripFileScheme(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}
