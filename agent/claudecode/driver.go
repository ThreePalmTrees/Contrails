package claudecode

import (
	"fmt"
	"path/filepath"
	"time"

	"contrails/agent"
)

// DirectoryWatcher abstracts filesystem watching so the driver doesn't
// depend on a concrete Watcher type from the main package.
type DirectoryWatcher interface {
	AddWatch(projectID, watchDir string) error
	RemoveWatch(watchDir string)
}

// Driver implements agent.AgentDriver for Claude Code.
// It manages the Stop hook lifecycle, signal watcher registration,
// and transcript directory watching for real-time session detection.
// Style: Verify Interface Compliance (go-style-guide.md)
var _ agent.AgentDriver = (*Driver)(nil)

// Driver is the Claude Code agent driver.
type Driver struct {
	signalWatcher    *SignalWatcher
	hookEnforcer     *HookEnforcer
	directoryWatcher DirectoryWatcher
	logger           agent.Logger
}

// NewDriver creates a Claude Code driver. The signalWatcher,
// hookEnforcer, and directoryWatcher may be nil if initialization
// failed — the driver degrades gracefully.
func NewDriver(signalWatcher *SignalWatcher, hookEnforcer *HookEnforcer, directoryWatcher DirectoryWatcher, logger agent.Logger) *Driver {
	return &Driver{
		signalWatcher:    signalWatcher,
		hookEnforcer:     hookEnforcer,
		directoryWatcher: directoryWatcher,
		logger:           logger,
	}
}

// Setup installs the Claude Code Stop hook into the project's
// .claude/settings.local.json so that session-end signals are emitted.
func (d *Driver) Setup(workspacePath string) error {
	if workspacePath == "" {
		return nil
	}
	if err := InstallHook(workspacePath); err != nil {
		agent.LogWarningf(d.logger, "Failed to install Claude Code hook: %v", err)
		// Non-fatal: likely a permissions issue on the workspace path.
		return nil
	}
	return nil
}

// Teardown uninstalls the Claude Code Stop hook.
func (d *Driver) Teardown(workspacePath string) error {
	if workspacePath == "" {
		return nil
	}
	if err := UninstallHook(workspacePath); err != nil {
		agent.LogWarningf(d.logger, "Failed to uninstall Claude Code hook: %v", err)
		return nil
	}
	return nil
}

// Activate registers the workspace path with the signal watcher and
// starts watching the transcript directory for new/changed session files.
func (d *Driver) Activate(projectID, workspacePath, watchDir string) error {
	if d.signalWatcher != nil && workspacePath != "" {
		d.signalWatcher.RegisterPath(workspacePath)
	}
	if d.hookEnforcer != nil && workspacePath != "" {
		d.hookEnforcer.Register(workspacePath)
	}
	// Watch the transcript directory so the frontend detects new sessions in real time
	if d.directoryWatcher != nil && watchDir != "" {
		if err := d.directoryWatcher.AddWatch(projectID, watchDir); err != nil {
			agent.LogWarningf(d.logger, "Failed to watch Claude Code transcript dir %s: %v", watchDir, err)
		}
	}
	return nil
}

// Deactivate unregisters the workspace path from the signal watcher
// and stops watching the transcript directory.
func (d *Driver) Deactivate(workspacePath, watchDir string) {
	if d.signalWatcher != nil && workspacePath != "" {
		d.signalWatcher.UnregisterPath(workspacePath)
	}
	if d.hookEnforcer != nil && workspacePath != "" {
		d.hookEnforcer.Unregister(workspacePath)
	}
	if d.directoryWatcher != nil && watchDir != "" {
		d.directoryWatcher.RemoveWatch(watchDir)
	}
}

// ProcessAll processes all .jsonl session files in the transcript directory,
// parses each session, and writes contrail markdown files.
func (d *Driver) ProcessAll(sourceDir, outputDir string, callbacks agent.ProcessCallbacks) (int, error) {
	sessionFiles, err := ListSessionFiles(sourceDir)
	if err != nil {
		return 0, fmt.Errorf("listing session files: %w", err)
	}

	total := len(sessionFiles)
	if total == 0 {
		return 0, nil
	}

	parser := &Parser{}
	count := 0

	for i, sessionFile := range sessionFiles {
		if callbacks.OnProgress != nil {
			callbacks.OnProgress(i+1, total)
		}

		start := time.Now()
		session, err := parser.ParseFile(sessionFile)
		if err != nil {
			agent.LogWarningf(d.logger, "Error parsing Claude Code session %s: %v", filepath.Base(sessionFile), err)
			continue
		}

		// Skip empty sessions
		if len(session.Messages) == 0 {
			continue
		}

		outputPath, err := agent.WriteParsedSession(session, outputDir)
		if err != nil {
			agent.LogWarningf(d.logger, "Error writing contrail for %s: %v", filepath.Base(sessionFile), err)
			continue
		}

		durationMs := time.Since(start).Milliseconds()
		count++

		if callbacks.OnFileProcessed != nil {
			callbacks.OnFileProcessed(filepath.Base(outputPath), durationMs)
		}
	}

	return count, nil
}
