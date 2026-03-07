package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScannedProject represents a Claude Code project found during scanning.
type ScannedProject struct {
	// EncodedName is the directory name under ~/.claude/projects/ (e.g., "-Users-user-projects-foo").
	EncodedName string `json:"encodedName"`

	// ProjectPath is the decoded filesystem path (e.g., "/Users/user/projects/foo").
	ProjectPath string `json:"projectPath"`

	// DisplayName is the last component of ProjectPath, suitable for display.
	DisplayName string `json:"displayName"`

	// SessionCount is the number of .jsonl session files found.
	SessionCount int `json:"sessionCount"`

	// TranscriptDirectory is the full path to the project's Claude directory.
	TranscriptDirectory string `json:"transcriptDirectory"`
}

// BrowseProjects scans ~/.claude/projects/ for directories containing
// .jsonl session files. It reverses the path encoding to recover the
// original project path.
func BrowseProjects() ([]ScannedProject, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	projectsDirectory := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScannedProject{}, nil
		}
		return nil, fmt.Errorf("reading Claude projects directory: %w", err)
	}

	var projects []ScannedProject
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		transcriptDirectory := filepath.Join(projectsDirectory, entry.Name())

		// Count .jsonl files
		sessionCount := countJSONLFiles(transcriptDirectory)
		if sessionCount == 0 {
			continue // Skip directories without any sessions
		}

		projectPath := decodeProjectPath(entry.Name())

		projects = append(projects, ScannedProject{
			EncodedName:         entry.Name(),
			ProjectPath:         projectPath,
			DisplayName:         filepath.Base(projectPath),
			SessionCount:        sessionCount,
			TranscriptDirectory: transcriptDirectory,
		})
	}

	return projects, nil
}

// decodeProjectPath reverses the Claude Code path encoding.
// Claude Code encodes paths by replacing "/" (and "_") with "-", so
// "-Users-user-projects-foo" becomes "/Users/user/projects/foo".
//
// Because the encoding is lossy (both "/" and "_" map to "-"), we use
// filesystem lookups to find the correct interpretation. For each "-" between
// parts, we try joining with "_", "-", or treating it as a "/" separator,
// preferring whichever produces an existing path.
func decodeProjectPath(encodedName string) string {
	// Split on "-". The first element is always empty (leading "-" = leading "/").
	parts := strings.Split(encodedName, "-")
	if len(parts) < 2 {
		return "/" + encodedName
	}

	// Try filesystem-aware decode first; fall back to naive decode.
	if result, ok := resolvePathParts(parts[1:], "/"); ok {
		return result
	}

	// Fallback: naive decode (replace all "-" with "/")
	decoded := strings.ReplaceAll(encodedName, "-", "/")
	return filepath.Clean(decoded)
}

// resolvePathParts recursively tries to reconstruct the original path from
// the split parts by trying three interpretations for each "-":
//  1. "_" (underscore in original name)
//  2. "-" (literal dash in original name)
//  3. "/" (directory separator)
//
// It uses filesystem existence checks to find the correct interpretation.
func resolvePathParts(parts []string, prefix string) (string, bool) {
	if len(parts) == 0 {
		return prefix, pathExists(prefix)
	}

	// Start with the first part as the current component
	current := parts[0]

	for i := 1; i <= len(parts); i++ {
		if i == len(parts) {
			// All parts consumed into current component — check as final path element.
			candidate := filepath.Join(prefix, current)
			if pathExists(candidate) {
				return candidate, true
			}
		} else {
			// Try joining with "_" first (underscore was encoded as "-").
			// We prefer "_" over "/" because underscore-named dirs are common
			// and the "/" interpretation can produce false positives when a
			// parent directory happens to exist with the same prefix.
			withUnderscore := current + "_" + parts[i]
			candidateU := filepath.Join(prefix, withUnderscore)
			if i == len(parts)-1 {
				if pathExists(candidateU) {
					return candidateU, true
				}
			} else {
				infoU, errU := os.Stat(candidateU)
				if errU == nil && infoU.IsDir() {
					if result, ok := resolvePathParts(parts[i+1:], candidateU); ok {
						return result, true
					}
				}
			}
			// Try treating this "-" as a "/" separator: current becomes a directory
			candidateDir := filepath.Join(prefix, current)
			info, err := os.Stat(candidateDir)
			if err == nil && info.IsDir() {
				if result, ok := resolvePathParts(parts[i:], candidateDir); ok {
					return result, true
				}
			}
			// Continue accumulating with "-" (literal dash)
			current = current + "-" + parts[i]
		}
	}

	return "", false
}

// pathExists checks whether the given path exists on the filesystem.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// countJSONLFiles counts the number of .jsonl files in a directory.
func countJSONLFiles(directory string) int {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			count++
		}
	}
	return count
}

// ListSessionFiles returns the paths of all .jsonl session files
// in a Claude Code project transcript directory.
func ListSessionFiles(transcriptDirectory string) ([]string, error) {
	entries, err := os.ReadDir(transcriptDirectory)
	if err != nil {
		return nil, fmt.Errorf("reading transcript directory: %w", err)
	}

	var sessionFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			sessionFiles = append(sessionFiles, filepath.Join(transcriptDirectory, entry.Name()))
		}
	}
	return sessionFiles, nil
}
