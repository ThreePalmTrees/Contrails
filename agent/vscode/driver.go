package vscode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"contrails/agent"
)

// DirectoryWatcher abstracts filesystem watching so the driver doesn't
// depend on a concrete Watcher type from the main package.
type DirectoryWatcher interface {
	AddWatch(projectID, watchDir string) error
	RemoveWatch(watchDir string)
}

// Driver implements agent.AgentDriver for VSCode Copilot.
// It manages fsnotify-based directory watching via the DirectoryWatcher.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.AgentDriver = (*Driver)(nil)

// Driver is the VSCode Copilot agent driver.
type Driver struct {
	watcher DirectoryWatcher
	logger  agent.Logger
}

// NewDriver creates a VS Code driver. The watcher may be nil if
// filesystem watching failed to initialize — the driver degrades gracefully.
func NewDriver(watcher DirectoryWatcher, logger agent.Logger) *Driver {
	return &Driver{
		watcher: watcher,
		logger:  logger,
	}
}

// Setup is a no-op for VS Code — no one-time installation is needed.
func (d *Driver) Setup(workspacePath string) error {
	return nil
}

// Teardown is a no-op for VS Code — no cleanup is needed.
func (d *Driver) Teardown(workspacePath string) error {
	return nil
}

// Activate starts watching the chatSessions directory via fsnotify.
func (d *Driver) Activate(projectID, workspacePath, watchDir string) error {
	if d.watcher == nil || watchDir == "" {
		return nil
	}
	return d.watcher.AddWatch(projectID, watchDir)
}

// Deactivate stops watching the chatSessions directory.
func (d *Driver) Deactivate(workspacePath, watchDir string) {
	if d.watcher == nil || watchDir == "" {
		return
	}
	d.watcher.RemoveWatch(watchDir)
}

// ProcessAll processes all chat session JSONL files in sourceDir,
// parses each session, writes contrail markdown files to outputDir,
// and heals any stale filenames.
func (d *Driver) ProcessAll(sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return 0, fmt.Errorf("reading directory: %w", err)
	}

	// Collect session files
	var sessionFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && IsChatSessionFile(entry.Name()) {
			sessionFiles = append(sessionFiles, entry)
		}
	}

	total := len(sessionFiles)
	parser := &Parser{}
	count := 0

	for i, entry := range sessionFiles {
		if callbacks.OnProgress != nil {
			callbacks.OnProgress(i+1, total)
		}

		filePath := filepath.Join(sourceDir, entry.Name())

		if callbacks.ShouldSkip != nil && callbacks.ShouldSkip(filePath) {
			continue
		}

		start := time.Now()

		session, err := parser.ParseFile(filePath)
		if err != nil {
			agent.LogWarningf(d.logger, "Error processing %s: %v", entry.Name(), err)
			continue
		}

		// Skip empty sessions (no messages yet)
		if len(session.Messages) == 0 {
			continue
		}

		outputPath, err := agent.WriteParsedSession(session, outputDir)
		if err != nil {
			agent.LogWarningf(d.logger, "Error writing contrail for %s: %v", entry.Name(), err)
			continue
		}

		durationMs := time.Since(start).Milliseconds()
		count++

		if callbacks.OnFileProcessed != nil {
			callbacks.OnFileProcessed(filepath.Base(outputPath), durationMs)
		}
	}

	// Heal any stale filenames (session ID → title)
	if healed, _ := d.HealContrailNames(sourceDir, outputDir); healed > 0 {
		agent.LogInfof(d.logger, "Healed %d contrail filename(s)", healed)
	}

	return count, nil
}

// HealContrailNames scans the output directory for .md files that use session IDs
// as filenames and re-processes the corresponding chat session JSONL files so that
// any available customTitle is picked up and the file is renamed accordingly.
func (d *Driver) HealContrailNames(watchDir, outputDir string) (int, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading output dir: %w", err)
	}

	parser := &Parser{}
	healed := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Read the header to extract the session ID
		mdPath := filepath.Join(outputDir, entry.Name())
		head, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}

		// Extract session ID from the "- **Session ID:** `{uuid}`" line
		headerStr := string(head[:min(len(head), 512)])
		sessionID := extractSessionIDFromHeader(headerStr)
		if sessionID == "" {
			continue
		}

		// Check if the filename already uses a title (not the session ID)
		if !strings.Contains(entry.Name(), sessionID) {
			continue // Already has a title-based name
		}

		// Try to re-process the source file to pick up any title
		sourcePath := FindSourceFile(watchDir, sessionID)
		if sourcePath == "" {
			continue // Source file doesn't exist
		}

		session, err := parser.ParseFile(sourcePath)
		if err != nil {
			agent.LogWarningf(d.logger, "Heal: error re-processing %s: %v", sessionID, err)
			continue
		}

		if len(session.Messages) == 0 {
			continue
		}

		if _, err := agent.WriteParsedSession(session, outputDir); err != nil {
			agent.LogWarningf(d.logger, "Heal: error writing %s: %v", sessionID, err)
			continue
		}

		// Check if the old file was removed (meaning it got renamed)
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			healed++
			agent.LogInfof(d.logger, "Healed contrail: %s", sessionID)
		}
	}

	return healed, nil
}

// extractSessionIDFromHeader pulls the session UUID from a contrail markdown header.
// Looks for: **Session ID:** `{uuid}`
func extractSessionIDFromHeader(header string) string {
	marker := "**Session ID:** `"
	idx := strings.Index(header, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.Index(header[start:], "`")
	if end < 0 {
		return ""
	}
	return header[start : start+end]
}
