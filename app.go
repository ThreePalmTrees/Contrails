package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"contrails/agent"
	"contrails/agent/claudecode"
	"contrails/agent/cursor"
	"contrails/agent/vscode"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx            context.Context
	watcher        *Watcher
	signalWatcher  *claudecode.SignalWatcher
	hookEnforcer   *claudecode.HookEnforcer
	logger         Logger
	emitter        EventEmitter
	dialogOpener   DialogOpener
	analytics      *Analytics
	drivers        map[AgentSourceType]agent.AgentDriver // source type → driver
	configDir      string           // override for testing; empty = os.UserConfigDir()
	lastFileHashes map[string]string // filePath → content hash at last processing
	// Guidelines: Don't fire-and-forget goroutines (go-style-guide.md)
	// Style: Descriptive Naming (go-style-guide.md)
	waitGroup sync.WaitGroup
}

// NewApp creates a new App application struct.
// Logger and EventEmitter default to no-ops so the App is safe to use
// in unit tests without a live Wails context. Production implementations
// are wired in during startup.
func NewApp() *App {
	return &App{
		logger:        &NoopLogger{},
		emitter:       &NoopEmitter{},
		drivers:       make(map[AgentSourceType]agent.AgentDriver),
		lastFileHashes: make(map[string]string),
	}
}

// startup is called when the app starts
func (app *App) startup(ctx context.Context) {
	app.ctx = ctx

	// Wire production Wails implementations
	app.logger = &WailsLogger{ctx: ctx}
	app.emitter = &WailsEventEmitter{ctx: ctx}
	app.dialogOpener = &WailsDialogOpener{ctx: ctx}

	// Style: Descriptive Naming (go-style-guide.md)
	newWatcher, err := NewWatcher(app.logger, app.emitter)
	if err != nil {
		app.logger.Error(fmt.Sprintf("Failed to create watcher: %v", err))
		return
	}
	app.watcher = newWatcher
	app.watcher.Start(ctx)

	// Start Claude Code signal watcher
	signalWatcher, err := claudecode.NewSignalWatcher(app.logger, app)
	if err != nil {
		app.logger.Warning(fmt.Sprintf("Failed to create signal watcher: %v", err))
	} else {
		app.signalWatcher = signalWatcher
		app.signalWatcher.Start(ctx)
	}

	// Start hook enforcer to keep the Stop hook installed while the app is running
	app.hookEnforcer = claudecode.NewHookEnforcer(app.logger)
	app.hookEnforcer.Start(ctx)

	// Create agent drivers (OCP: new agents register here, no other changes needed)
	app.drivers[AgentSourceVSCode] = vscode.NewDriver(app.watcher, app.logger)
	app.drivers[AgentSourceClaudeCode] = claudecode.NewDriver(app.signalWatcher, app.hookEnforcer, app.watcher, app.logger)
	app.drivers[AgentSourceCursor] = cursor.NewDriver(app, app.logger)

	// Initialize analytics (no-ops if PostHogAPIKey is empty)
	app.analytics = NewAnalytics(PostHogAPIKey, app.logger, app.GetProjects)

	// Check opt-out setting
	if settings, err := app.loadSettings(); err == nil && !settings.AnalyticsEnabled {
		app.analytics.SetEnabled(false)
	}

	// Re-watch all projects and heal stale filenames
	projects, _ := app.GetProjects()
	for _, project := range projects {
		// Re-run Setup so hooks installed after project creation are applied (e.g. Claude Code Stop hook)
		for _, source := range project.Sources {
			if driver, ok := app.drivers[source.Type]; ok {
				if err := driver.Setup(project.WorkspacePath); err != nil {
					logWarningf(app.logger, "Failed to set up %s for %s: %v", source.Type, project.Name, err)
				}
			}
		}
		app.activateProjectSources(project)

		if project.Active {
			// Heal VSCode contrails with stale filenames (e.g., session ID instead of title)
			if project.WatchDir != "" {
				if vsDriver, ok := app.drivers[AgentSourceVSCode].(*vscode.Driver); ok {
					if healed, _ := vsDriver.HealContrailNames(project.WatchDir, project.OutputDir); healed > 0 {
						logInfof(app.logger, "Healed %d contrail filename(s) for %s", healed, project.Name)
					}
				}
			}
		}
	}

	// Guidelines: Don't fire-and-forget goroutines — track with WaitGroup (go-style-guide.md)
	app.waitGroup.Add(1)
	go func() {
		defer app.waitGroup.Done()
		for _, project := range projects {
			if !project.Active || project.LastProcessed == 0 {
				continue
			}
			// ProcessModifiedSince is VSCode-specific; find the VSCode watch dir
			watchDir := project.WatchDir
			for _, source := range project.Sources {
				if source.Type == AgentSourceVSCode {
					watchDir = effectiveWatchDir(source, project)
					break
				}
			}
			if watchDir == "" {
				continue
			}
			count, err := app.ProcessModifiedSince(project.ID, watchDir, project.OutputDir)
			if err != nil {
				logWarningf(app.logger, "Startup scan error for %s: %v", project.Name, err)
				app.emitter.Emit("app:error", AppError{
					ProjectID:   project.ID,
					ProjectName: project.Name,
					Message:     fmt.Sprintf("Startup scan failed: %v", err),
				})
			} else if count > 0 {
				logInfof(app.logger, "Startup: processed %d modified file(s) for %s", count, project.Name)
			}
		}
	}()

	app.analytics.TrackAppStarted()

	// Clean up any .app.old from a previous update
	CleanupOldUpdate()

	// Check for updates in the background (non-blocking)
	app.waitGroup.Add(1)
	go func() {
		defer app.waitGroup.Done()
		time.Sleep(3 * time.Second) // Delay so startup isn't slowed
		if info, _ := CheckForUpdate(); info != nil {
			app.emitter.Emit("update:available", info)
		}
	}()
}

// shutdown is called when the app is closing
func (app *App) shutdown(ctx context.Context) {
	// Guidelines: Don't fire-and-forget goroutines — wait for background work (go-style-guide.md)
	app.waitGroup.Wait()
	if app.watcher != nil {
		app.watcher.Stop()
	}
	if app.signalWatcher != nil {
		app.signalWatcher.Stop()
	}
	if app.hookEnforcer != nil {
		app.hookEnforcer.Stop()
	}

	app.analytics.TrackAppClosed()
	app.analytics.Close()
}

// --- SignalHandler implementation (Dependency Inversion) ---
// App implements claudecode.SignalHandler so the signal watcher can call
// back into the application layer without depending on concrete types.
var _ claudecode.SignalHandler = (*App)(nil)

// FindProject matches a working directory to a registered project.
// It checks for exact match on WorkspacePath, then prefix match (the cwd
// may be a subdirectory of the workspace).
func (app *App) FindProject(workingDirectory string) (projectID, outputDir string, found bool) {
	projects, err := app.GetProjects()
	if err != nil {
		return "", "", false
	}

	// Exact match first
	for _, p := range projects {
		if p.WorkspacePath == workingDirectory && hasSource(p, AgentSourceClaudeCode) {
			return p.ID, p.OutputDir, true
		}
	}

	// Prefix match: the cwd might be a subdirectory of the workspace
	for _, p := range projects {
		if hasSource(p, AgentSourceClaudeCode) && strings.HasPrefix(workingDirectory, p.WorkspacePath+"/") {
			return p.ID, p.OutputDir, true
		}
	}

	return "", "", false
}

// WriteContrail writes the parsed session as a markdown file and returns
// the output path. Delegates to the shared WriteParsedSession function.
func (app *App) WriteContrail(session *agent.ParsedSession, outputDir string) (string, error) {
	path, err := agent.WriteParsedSession(session, outputDir)
	if err == nil && path != "" {
		app.analytics.TrackContrailCreated(session.Agent, len(session.Messages))
	}
	return path, err
}

// EmitWatcherEvent notifies the frontend that a contrail file was
// created, modified, or removed.
func (app *App) EmitWatcherEvent(projectID, fileName, eventType string) {
	app.emitter.Emit("watcher:event", WatcherEvent{
		ProjectID: projectID,
		FileName:  fileName,
		EventType: eventType,
	})
}

// EmitFileProcessed notifies the frontend that a source file was processed.
func (app *App) EmitFileProcessed(projectID, fileName string) {
	app.emitter.Emit("file:processed", FileProcessedEvent{
		ProjectID: projectID,
		FileName:  fileName,
	})
}

// --- Project Management ---

// Errors: Handle Errors Once — propagate instead of silently discarding (go-style-guide.md)
func (app *App) projectsFilePath() (string, error) {
	baseDir := app.configDir
	if baseDir == "" {
		var err error
		baseDir, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("getting config dir: %w", err)
		}
	}
	dir := filepath.Join(baseDir, "contrails")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating config dir: %w", err)
	}
	return filepath.Join(dir, "projects.json"), nil
}

// GetProjects returns all saved projects
func (app *App) GetProjects() ([]Project, error) {
	path, err := app.projectsFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Project{}, nil
		}
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// SaveProjects persists the projects list
func (app *App) SaveProjects(projects []Project) error {
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	path, err := app.projectsFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AddProject creates a new project and sets up watchers/hooks for its sources.
func (app *App) AddProject(project Project) error {
	projects, err := app.GetProjects()
	if err != nil {
		return err
	}
	project.Active = true
	project.LastProcessed = time.Now().UnixMilli()
	projects = append(projects, project)

	if err := app.SaveProjects(projects); err != nil {
		return err
	}

	// Setup and activate all agent sources via drivers
	for _, source := range project.Sources {
		if driver, ok := app.drivers[source.Type]; ok {
			if err := driver.Setup(project.WorkspacePath); err != nil {
				logWarningf(app.logger, "Failed to set up %s for %s: %v", source.Type, project.Name, err)
			}
			watchDir := effectiveWatchDir(source, project)
			if err := driver.Activate(project.ID, project.WorkspacePath, watchDir); err != nil {
				return fmt.Errorf("activating %s: %w", source.Type, err)
			}
		}
	}
	// Legacy: projects without Sources but with WatchDir
	if len(project.Sources) == 0 && project.WatchDir != "" {
		if driver, ok := app.drivers[AgentSourceVSCode]; ok {
			if err := driver.Activate(project.ID, project.WorkspacePath, project.WatchDir); err != nil {
				return fmt.Errorf("starting watch: %w", err)
			}
		}
	}

	// Analytics: track project addition
	agentTypes := make([]string, 0, len(project.Sources))
	for _, s := range project.Sources {
		agentTypes = append(agentTypes, string(s.Type))
	}
	app.analytics.TrackProjectAdded(agentTypes, len(projects))

	return nil
}

// UpdateProject updates an existing project
func (app *App) UpdateProject(project Project) error {
	projects, err := app.GetProjects()
	if err != nil {
		return err
	}

	for i, existing := range projects {
		if existing.ID == project.ID {
			// Update agent drivers if watch dir or sources changed
			// Keep watchers running even when paused so new raw chat sessions
			// show up in the UI list, but we'll stop parsing in ProcessFileIfNeeded.
			sourcesChanged := existing.WatchDir != project.WatchDir || len(existing.Sources) != len(project.Sources)
			if !sourcesChanged {
				for j, src := range existing.Sources {
					if src.Type != project.Sources[j].Type || src.WatchDir != project.Sources[j].WatchDir {
						sourcesChanged = true
						break
					}
				}
			}

			if sourcesChanged {
				app.deactivateProjectSources(existing)
				app.activateProjectSources(project)

				// Analytics: track source additions/removals
				oldTypes := map[AgentSourceType]bool{}
				for _, s := range existing.Sources {
					oldTypes[s.Type] = true
				}
				newTypes := map[AgentSourceType]bool{}
				for _, s := range project.Sources {
					newTypes[s.Type] = true
				}
				for t := range newTypes {
					if !oldTypes[t] {
						app.analytics.TrackAgentSourceAdded(string(t))
					}
				}
				for t := range oldTypes {
					if !newTypes[t] {
						app.analytics.TrackAgentSourceRemoved(string(t))
					}
				}
			}

			// On resume: process files modified since pause
			if !existing.Active && project.Active && existing.LastProcessed > 0 && project.WatchDir != "" {
				// Guidelines: Don't fire-and-forget goroutines — track with WaitGroup (go-style-guide.md)
				app.waitGroup.Add(1)
				go func(projectID, watchDir, outputDir string) {
					defer app.waitGroup.Done()
					count, err := app.ProcessModifiedSince(projectID, watchDir, outputDir)
					if err != nil {
						logWarningf(app.logger, "Resume scan error: %v", err)
					} else if count > 0 {
						logInfof(app.logger, "Resume: processed %d modified file(s)", count)
					}
				}(project.ID, project.WatchDir, project.OutputDir)
			}

			// Set PausedAt when pausing
			if existing.Active && !project.Active {
				project.PausedAt = time.Now().UnixMilli()
			} else if !existing.Active && project.Active {
				project.PausedAt = 0 // Clear on resume
			} else {
				project.PausedAt = existing.PausedAt // Preserve existing
			}
			projects[i] = project
			return app.SaveProjects(projects)
		}
	}
	return fmt.Errorf("project not found: %s", project.ID)
}

// RemoveProject removes a project and cleans up its watchers/hooks.
func (app *App) RemoveProject(projectID string) error {
	projects, err := app.GetProjects()
	if err != nil {
		return err
	}

	var updated []Project
	for _, project := range projects {
		if project.ID == projectID {
			// Deactivate and teardown all agent sources via drivers
			app.deactivateProjectSources(project)
			app.teardownProjectSources(project)
			continue
		}
		updated = append(updated, project)
	}

	if err := app.SaveProjects(updated); err != nil {
		return err
	}

	app.analytics.TrackProjectRemoved(len(updated))
	return nil
}

// --- File/Directory Selection ---

// SelectChatSessionsDir opens a native directory picker
func (app *App) SelectChatSessionsDir() (string, error) {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")

	result, err := app.dialogOpener.OpenDirectoryDialog(wailsRuntime.OpenDialogOptions{
		Title:            "Select chatSessions directory",
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// SelectOutputDir opens a native directory picker for output
func (app *App) SelectOutputDir() (string, error) {
	result, err := app.dialogOpener.OpenDirectoryDialog(wailsRuntime.OpenDialogOptions{
		Title: "Select output directory",
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// BrowseWorkspaceStorages lists available workspace storage directories
func (app *App) BrowseWorkspaceStorages() ([]map[string]string, error) {
	home, _ := os.UserHomeDir()
	storageDir := filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil, fmt.Errorf("reading workspaceStorage: %w", err)
	}

	var results []map[string]string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(storageDir, entry.Name())
		chatSessionsDir := filepath.Join(dirPath, "chatSessions")

		// Only include directories that have a chatSessions subdirectory
		if _, err := os.Stat(chatSessionsDir); os.IsNotExist(err) {
			continue
		}

		info := map[string]string{
			"id":              entry.Name(),
			"chatSessionsDir": chatSessionsDir,
			"name":            entry.Name(), // default to UUID
		}

		// Try to read workspace.json for project name
		workspaceJSONPath := filepath.Join(dirPath, "workspace.json")
		if data, err := os.ReadFile(workspaceJSONPath); err == nil {
			var workspaceConfig map[string]interface{}
			if err := json.Unmarshal(data, &workspaceConfig); err == nil {
				if folder, ok := workspaceConfig["folder"].(string); ok {
					// Extract project name from folder URI
					cleaned := strings.TrimPrefix(folder, "file://")
					info["name"] = filepath.Base(cleaned)
					info["workspacePath"] = cleaned
				} else if workspace, ok := workspaceConfig["workspace"].(string); ok {
					// Extract project name from .code-workspace file URI
					cleaned := strings.TrimPrefix(workspace, "file://")
					info["name"] = filepath.Base(cleaned)
					info["workspacePath"] = cleaned
				}
			}
		}

		results = append(results, info)
	}

	return results, nil
}

// --- Processing ---

// makeProcessCallbacks builds the ProcessCallbacks that wrap progress
// events and per-file duration/badge tracking for the given project.
func (app *App) makeProcessCallbacks(projectID string) agent.ProcessCallbacks {
	return agent.ProcessCallbacks{
		OnProgress: func(current, total int) {
			app.emitter.Emit("processing:progress", ProcessingProgress{
				ProjectID: projectID,
				Current:   current,
				Total:     total,
			})
		},
		OnFileProcessed: func(outputFileName string, durationMs int64) {
			app.emitter.Emit("file:processed", FileProcessedEvent{
				ProjectID: projectID,
				FileName:  outputFileName,
			})
		},
	}
}

// --- Analytics Settings ---

// AnalyticsSettings holds the opt-out preference, persisted alongside projects.json.
type AnalyticsSettings struct {
	AnalyticsEnabled bool `json:"analyticsEnabled"`
}

func (app *App) settingsFilePath() (string, error) {
	baseDir := app.configDir
	if baseDir == "" {
		var err error
		baseDir, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("getting config dir: %w", err)
		}
	}
	dir := filepath.Join(baseDir, "contrails")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating config dir: %w", err)
	}
	return filepath.Join(dir, "settings.json"), nil
}

func (app *App) loadSettings() (AnalyticsSettings, error) {
	path, err := app.settingsFilePath()
	if err != nil {
		return AnalyticsSettings{AnalyticsEnabled: true}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Default: analytics enabled
		return AnalyticsSettings{AnalyticsEnabled: true}, nil
	}
	var settings AnalyticsSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return AnalyticsSettings{AnalyticsEnabled: true}, err
	}
	return settings, nil
}

func (app *App) saveSettings(settings AnalyticsSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	path, err := app.settingsFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetAnalyticsEnabled returns whether analytics collection is enabled.
func (app *App) GetAnalyticsEnabled() bool {
	settings, err := app.loadSettings()
	if err != nil {
		return true // default enabled
	}
	return settings.AnalyticsEnabled
}

// SetAnalyticsEnabled toggles analytics collection on or off.
func (app *App) SetAnalyticsEnabled(enabled bool) error {
	settings, _ := app.loadSettings()
	settings.AnalyticsEnabled = enabled
	if err := app.saveSettings(settings); err != nil {
		return err
	}
	app.analytics.SetEnabled(enabled)
	return nil
}

// --- Update ---

// CheckForAppUpdate checks GitHub for a newer version. Returns nil if up to date.
func (app *App) CheckForAppUpdate() *UpdateInfo {
	info, _ := CheckForUpdate()
	return info
}

// ApplyAppUpdate downloads and installs the update, then relaunches.
func (app *App) ApplyAppUpdate(downloadURL string) error {
	app.analytics.Track("update_accepted", map[string]interface{}{
		"from_version": Version,
	})
	app.analytics.Close() // Flush before we exit
	return ApplyUpdate(downloadURL)
}

// GetVersion returns the current app version.
func (app *App) GetVersion() string {
	return Version
}

// ProcessChatSessions processes all JSON files in a chat sessions directory.
// This is the "Process All Now" path - ignores lastProcessedAt.
// Delegates to the VS Code driver's ProcessAll.
func (app *App) ProcessChatSessions(projectID, watchDir, outputDir string) (int, error) {
	driver, ok := app.drivers[AgentSourceVSCode]
	if !ok {
		return 0, fmt.Errorf("no VS Code driver registered")
	}

	count, err := driver.ProcessAll(watchDir, outputDir, app.makeProcessCallbacks(projectID))
	if err != nil {
		return 0, err
	}

	// Update lastProcessedAt
	if err := app.updateLastProcessed(projectID); err != nil {
		logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
	}

	if count > 0 {
		app.analytics.TrackProcessAll("vscode", count)
	}

	return count, nil
}

// ProcessSingleFile processes a single chat session file.
// sourceType must be "vscode", "claudecode", or "cursor".
func (app *App) ProcessSingleFile(filePath, sourceType, outputDir string) (string, error) {
	result, _, err := app.processFileWithSource(filePath, sourceType, outputDir)
	return result, err
}

// ProcessFileIfNeeded processes a single file only if its lastMessageDate
// (from the JSON content) is newer than the project's lastProcessed timestamp.
// Uses content-based comparison instead of filesystem mtime, since VS Code
// frequently touches chat session files without adding new messages.
func (app *App) ProcessFileIfNeeded(projectID, filePath, outputDir string) (string, error) {
	// Get the project's lastProcessedAt
	projects, err := app.GetProjects()
	if err != nil {
		return "", err
	}

	var lastProcessed int64
	var isActive bool
	for _, project := range projects {
		if project.ID == projectID {
			lastProcessed = project.LastProcessed
			isActive = project.Active
			break
		}
	}

	// We're keeping watchers running to list raw files, but if the project
	// is paused, we don't automatically parse them.
	if !isActive {
		return "", nil
	}

	// Check lastMessageDate from JSON content instead of filesystem mtime.
	// VS Code frequently touches chat session files (updating mtime) without
	// adding new messages, which would cause unnecessary re-processing.
	lastMessageDate, err := vscode.ExtractLastMessageDate(filePath)
	if err != nil {
		return "", fmt.Errorf("extracting lastMessageDate: %w", err)
	}

	// Also check content hash — VS Code writes chat session files incrementally
	// during response streaming, so lastMessageDate may stay the same while
	// content grows (new response text, customTitle being set, etc.).
	// We use ExtractSessionSignature instead of file size to avoid false triggers
	// when VS Code only updates superficial metadata fields.
	currentHash, err := vscode.ExtractSessionSignature(filePath)
	if err != nil {
		currentHash = ""
	}
	previousHash := app.lastFileHashes[filePath]

	if lastMessageDate <= lastProcessed && currentHash == previousHash {
		// No new messages and content unchanged - skip
		return "", nil
	}

	result, _, err := app.processFileReturn(filePath, outputDir)
	if err != nil {
		return "", err
	}

	// Record the file hash so we can detect future content changes using the hash from BEFORE processing
	app.lastFileHashes[filePath] = currentHash

	// Update lastProcessedAt
	if err := app.updateLastProcessed(projectID); err != nil {
		logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
	}

	// Emit file-processed event for badge tracking (only if something was actually written)
	if result != "" {
		app.emitter.Emit("file:processed", FileProcessedEvent{
			ProjectID: projectID,
			FileName:  filepath.Base(filePath),
		})
	}

	return result, nil
}

// HandleDeletedFile marks the corresponding .md file as deleted
// when a chat session JSONL file is removed from the watch directory.
func (app *App) HandleDeletedFile(fileName, outputDir string) error {
	// The fileName is like {uuid}.jsonl - the sessionId is inside the file,
	// but the file is gone. We need to find the .md file that references
	// this session. We'll search by scanning existing .md files for the
	// session ID (which is the uuid from the filename without .jsonl or .json).
	sessionID := strings.TrimSuffix(strings.TrimSuffix(fileName, ".jsonl"), ".json")

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // output dir doesn't exist, nothing to do
		}
		return fmt.Errorf("reading output dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		mdPath := filepath.Join(outputDir, entry.Name())
		content, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}

		// Check if this .md file contains the session ID
		if !strings.Contains(string(content[:min(len(content), 512)]), sessionID) {
			continue
		}

		// Found it - prepend the deleted banner if not already present
		deletedBanner := "> ⚠️ **This chat session has been deleted.** The source file is no longer available.\n\n"
		if strings.HasPrefix(string(content), "> ⚠️ **This chat session has been deleted") {
			return nil // already flagged
		}

		newContent := deletedBanner + string(content)
		if err := os.WriteFile(mdPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("writing deleted banner: %w", err)
		}

		logInfof(app.logger, "Marked session %s as deleted in %s", sessionID, entry.Name())
		return nil
	}

	// No matching .md file found - that's fine, nothing to flag
	return nil
}

func (app *App) processFile(filePath, outputDir string) (time.Duration, error) {
	_, duration, err := app.processFileReturn(filePath, outputDir)
	return duration, err
}

// processFileWithSource parses and writes a session using the correct parser
// for the given sourceType ("vscode", "claudecode", or "cursor").
func (app *App) processFileWithSource(filePath, sourceType, outputDir string) (string, time.Duration, error) {
	start := time.Now()

	var parsed *agent.ParsedSession
	var err error
	switch AgentSourceType(sourceType) {
	case AgentSourceClaudeCode:
		p := &claudecode.Parser{}
		parsed, err = p.ParseFile(filePath)
	case AgentSourceCursor:
		p := &cursor.Parser{}
		parsed, err = p.ParseFile(filePath)
	default:
		p := &vscode.Parser{}
		parsed, err = p.ParseFile(filePath)
	}
	if err != nil {
		return "", 0, err
	}

	if len(parsed.Messages) == 0 {
		return "", time.Since(start), nil
	}

	outPath, err := agent.WriteParsedSession(parsed, outputDir)
	if err != nil {
		return "", 0, err
	}

	app.analytics.TrackContrailCreated(parsed.Agent, len(parsed.Messages))
	return outPath, time.Since(start), nil
}

func (app *App) processFileReturn(filePath, outputDir string) (string, time.Duration, error) {
	start := time.Now()

	// processFileReturn is only called from VS Code watcher paths.
	var parsed *agent.ParsedSession
	var err error
	parser := &vscode.Parser{}
	parsed, err = parser.ParseFile(filePath)
	if err != nil {
		return "", 0, err
	}

	// Skip empty sessions (no messages yet — user opened chat but hasn't typed)
	if len(parsed.Messages) == 0 {
		return "", time.Since(start), nil
	}

	outPath, err := agent.WriteParsedSession(parsed, outputDir)
	if err != nil {
		return "", 0, err
	}

	app.analytics.TrackContrailCreated(parsed.Agent, len(parsed.Messages))
	duration := time.Since(start)
	return outPath, duration, nil
}

// HealContrailNames scans the output directory for .md files that use session IDs
// as filenames and re-processes the corresponding chat session JSON files so that
// any available customTitle is picked up and the file is renamed accordingly.
func (app *App) HealContrailNames(watchDir, outputDir string) (int, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading output dir: %w", err)
	}

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
		sessionID := extractSessionID(headerStr)
		if sessionID == "" {
			continue
		}

		// Check if the filename already uses a title (not the session ID)
		if !strings.Contains(entry.Name(), sessionID) {
			continue // Already has a title-based name
		}

		// Try to re-process the source file to pick up any title
		sourcePath := vscode.FindSourceFile(watchDir, sessionID)
		if sourcePath == "" {
			continue // Source file doesn't exist
		}

		if _, _, err := app.processFileReturn(sourcePath, outputDir); err != nil {
			logWarningf(app.logger, "Heal: error re-processing %s: %v", sessionID, err)
			continue
		}

		// Check if the old file was removed (meaning it got renamed)
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			healed++
			logInfof(app.logger, "Healed contrail: %s", sessionID)
		}
	}

	return healed, nil
}

// extractSessionID pulls the session UUID from a contrail markdown header.
// Looks for: **Session ID:** `{uuid}`
func extractSessionID(header string) string {
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

// updateLastProcessed updates a project's LastProcessed timestamp to now.
// Errors: Handle Errors Once — propagate errors to caller (go-style-guide.md)
func (app *App) updateLastProcessed(projectID string) error {
	projects, err := app.GetProjects()
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i, project := range projects {
		if project.ID == projectID {
			projects[i].LastProcessed = now
			return app.SaveProjects(projects)
		}
	}
	return nil
}

// GetDefaultOutputDir returns the default contrails path for a workspace
func (app *App) GetDefaultOutputDir(workspacePath string) string {
	if workspacePath == "" {
		return ""
	}
	// For workspace files (e.g. .code-workspace), use the parent directory
	if info, err := os.Stat(workspacePath); err == nil && !info.IsDir() {
		workspacePath = filepath.Dir(workspacePath)
	}
	return filepath.Join(workspacePath, "contrails")
}

// ValidateOutputDir checks if the output directory is writable.
// Returns an empty string if valid, or an error message if not.
func (app *App) ValidateOutputDir(dirPath string) string {
	// If the directory exists, try writing a temp file
	if info, err := os.Stat(dirPath); err == nil {
		if !info.IsDir() {
			return "Path exists but is not a directory"
		}
		testFile := filepath.Join(dirPath, ".contrails_write_test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return "Directory is not writable"
		}
		os.Remove(testFile)
		return ""
	}

	// Directory doesn't exist - check if we can create it
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "Cannot create directory: " + err.Error()
	}
	// Clean up the test directory (only remove what we created)
	os.Remove(dirPath)
	return ""
}


// ListChatFiles returns all source chat session files for a project with their
// parsed status. A file is considered parsed if any .md file in the output
// directory contains the file's session ID in its first 512 bytes.
func (app *App) ListChatFiles(projectID string) ([]ChatFileInfo, error) {
	projects, err := app.GetProjects()
	if err != nil {
		return nil, err
	}

	var project *Project
	for i := range projects {
		if projects[i].ID == projectID {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Build map of session ID → output .md mtime (ms) for already-parsed files
	parsedAt := make(map[string]int64)
	if entries, err := os.ReadDir(project.OutputDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			mdPath := filepath.Join(project.OutputDir, entry.Name())
			head, err := os.ReadFile(mdPath)
			if err != nil {
				continue
			}
			sessionID := extractSessionID(string(head[:min(len(head), 512)]))
			if sessionID == "" {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				parsedAt[sessionID] = 1 // mark as parsed even if mtime unavailable
				continue
			}
			parsedAt[sessionID] = info.ModTime().UnixMilli()
		}
	}

	var files []ChatFileInfo

	for _, source := range project.Sources {
		watchDir := effectiveWatchDir(source, *project)
		if watchDir == "" {
			continue
		}

		switch source.Type {
		case AgentSourceVSCode:
			entries, err := os.ReadDir(watchDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() || !vscode.IsChatSessionFile(entry.Name()) {
					continue
				}
				filePath := filepath.Join(watchDir, entry.Name())
				sessionID := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".jsonl"), ".json")

				lastMessageDate, _ := vscode.ExtractLastMessageDate(filePath)
				firstMessageDate, _ := vscode.ExtractFirstMessageDate(filePath)
				mdMtime := parsedAt[sessionID]

				isParsed := mdMtime > 0
				isPartiallyParsed := isParsed && lastMessageDate > mdMtime

				info := ChatFileInfo{
					FileName:        entry.Name(),
					FilePath:        filePath,
					SourceType:      string(AgentSourceVSCode),
					Parsed:          isParsed && !isPartiallyParsed, // Complete if parsed and NOT partially parsed
					PartiallyParsed: isPartiallyParsed,
					ProcessedAt:     mdMtime,
					Title:           vscode.ExtractTitle(filePath),
					CreatedAt:       firstMessageDate,
				}
				files = append(files, info)
			}

		case AgentSourceClaudeCode:
			sessionFiles, err := claudecode.ListSessionFiles(watchDir)
			if err != nil {
				continue
			}
			for _, filePath := range sessionFiles {
				name := filepath.Base(filePath)
				sessionID := strings.TrimSuffix(name, ".jsonl")

				title := claudecode.ExtractTitle(filePath)
				// Skip empty sessions (opened and closed without any user message)
				if title == "" {
					continue
				}

				lastMessageDate, _ := claudecode.ExtractLastMessageDate(filePath)
				firstMessageDate, _ := claudecode.ExtractFirstMessageDate(filePath)
				mdMtime := parsedAt[sessionID]

				isParsed := mdMtime > 0
				isPartiallyParsed := isParsed && lastMessageDate > mdMtime

				info := ChatFileInfo{
					FileName:        name,
					FilePath:        filePath,
					SourceType:      string(AgentSourceClaudeCode),
					Parsed:          isParsed && !isPartiallyParsed,
					PartiallyParsed: isPartiallyParsed,
					ProcessedAt:     mdMtime,
					Title:           title,
					CreatedAt:       firstMessageDate,
				}
				files = append(files, info)
			}

		case AgentSourceCursor:
			composers, err := cursor.ListComposers(watchDir)
			if err != nil {
				continue
			}
			for _, c := range composers {
				mdMtime := parsedAt[c.ID]

				isParsed := mdMtime > 0
				isPartiallyParsed := isParsed && c.LastUpdatedAt > mdMtime

				info := ChatFileInfo{
					FileName:        c.ID,
					FilePath:        c.ID, // composer UUID used as identifier
					SourceType:      string(AgentSourceCursor),
					Parsed:          isParsed && !isPartiallyParsed,
					PartiallyParsed: isPartiallyParsed,
					ProcessedAt:     mdMtime,
					Title:           c.Name,
					LastMessageAt:   agent.FormatTimestamp(c.LastUpdatedAt),
					CreatedAt:       c.CreatedAt,
				}
				files = append(files, info)
			}
		}
	}

	// Legacy: project with WatchDir but no Sources
	if len(project.Sources) == 0 && project.WatchDir != "" {
		entries, err := os.ReadDir(project.WatchDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !vscode.IsChatSessionFile(entry.Name()) {
					continue
				}
				filePath := filepath.Join(project.WatchDir, entry.Name())
				sessionID := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".jsonl"), ".json")

				lastMessageDate, _ := vscode.ExtractLastMessageDate(filePath)
				firstMessageDate, _ := vscode.ExtractFirstMessageDate(filePath)
				mdMtime := parsedAt[sessionID]

				isParsed := mdMtime > 0
				isPartiallyParsed := isParsed && lastMessageDate > mdMtime

				info := ChatFileInfo{
					FileName:        entry.Name(),
					FilePath:        filePath,
					SourceType:      string(AgentSourceVSCode),
					Parsed:          isParsed && !isPartiallyParsed,
					PartiallyParsed: isPartiallyParsed,
					ProcessedAt:     mdMtime,
					Title:           vscode.ExtractTitle(filePath),
					CreatedAt:       firstMessageDate,
				}
				files = append(files, info)
			}
		}
	}

	return files, nil
}

// PreviewChatFile parses a single chat session file and returns the rendered
// markdown content as a string without writing anything to disk.
func (app *App) PreviewChatFile(filePath, sourceType string) (string, error) {
	var parsed *agent.ParsedSession
	var err error

	switch AgentSourceType(sourceType) {
	case AgentSourceClaudeCode:
		p := &claudecode.Parser{}
		parsed, err = p.ParseFile(filePath)
	case AgentSourceCursor:
		p := &cursor.Parser{}
		parsed, err = p.ParseFile(filePath)
	default:
		p := &vscode.Parser{}
		parsed, err = p.ParseFile(filePath)
	}
	if err != nil {
		return "", fmt.Errorf("parsing file: %w", err)
	}
	if parsed == nil || len(parsed.Messages) == 0 {
		return "", nil
	}

	return agent.RenderMarkdown(parsed), nil
}

// ReadExistingContrail reads the last generated markdown for a chat session.
// Returns an empty string if the file is not found.
func (app *App) ReadExistingContrail(fileName, outputDir string) (string, error) {
	sessionID := strings.TrimSuffix(strings.TrimSuffix(fileName, ".jsonl"), ".json")

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading output dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		mdPath := filepath.Join(outputDir, entry.Name())
		content, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}

		if strings.Contains(string(content[:min(len(content), 512)]), sessionID) {
			return string(content), nil
		}
	}

	return "", nil
}


// ProcessModifiedSince processes files modified after the project's lastProcessed.
// Used for startup scan and resume-after-pause. Emits progress and file-processed events.
func (app *App) ProcessModifiedSince(projectID, watchDir, outputDir string) (int, error) {
	projects, err := app.GetProjects()
	if err != nil {
		return 0, err
	}

	var lastProcessed int64
	for _, project := range projects {
		if project.ID == projectID {
			lastProcessed = project.LastProcessed
			break
		}
	}

	entries, err := os.ReadDir(watchDir)
	if err != nil {
		return 0, fmt.Errorf("reading directory: %w", err)
	}

	// Find files with new messages since lastProcessed.
	// Uses lastMessageDate from JSON content instead of filesystem mtime,
	// since VS Code frequently touches files without adding new messages.
	type pendingFile struct {
		path string
		name string
	}
	var pending []pendingFile

	for _, entry := range entries {
		if entry.IsDir() || !vscode.IsChatSessionFile(entry.Name()) {
			continue
		}
		filePath := filepath.Join(watchDir, entry.Name())
		lastMessageDate, err := vscode.ExtractLastMessageDate(filePath)
		if err != nil {
			continue
		}
		if lastMessageDate > lastProcessed {
			pending = append(pending, pendingFile{path: filePath, name: entry.Name()})
		}
	}

	if len(pending) == 0 {
		return 0, nil
	}

	total := len(pending)
	count := 0
	for i, file := range pending {
		// Emit progress
		app.emitter.Emit("processing:progress", ProcessingProgress{
			ProjectID: projectID,
			Current:   i + 1,
			Total:     total,
		})

		if _, err := app.processFile(file.path, outputDir); err != nil {
			logWarningf(app.logger, "Error processing %s: %v", file.name, err)
			continue
		}

		// Record the file hash so watcher-triggered ProcessFileIfNeeded
		// has a baseline to detect content changes during streaming
		if hash, err := vscode.ExtractSessionSignature(file.path); err == nil {
			app.lastFileHashes[file.path] = hash
		}

		count++

		// Emit file-processed event for badge tracking
		app.emitter.Emit("file:processed", FileProcessedEvent{
			ProjectID: projectID,
			FileName:  file.name,
		})
	}

	if count > 0 {
		if err := app.updateLastProcessed(projectID); err != nil {
			logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
		}
	}

	return count, nil
}

// GetWorkspaceStoragePath returns the default workspace storage path
func (app *App) GetWorkspaceStoragePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")
}

// --- Claude Code Integration ---

// BrowseClaudeCodeProjects scans ~/.claude/projects/ for Claude Code project
// directories containing .jsonl session files.
func (app *App) BrowseClaudeCodeProjects() ([]claudecode.ScannedProject, error) {
	return claudecode.BrowseProjects()
}

// --- Cursor Integration ---

// BrowseCursorProjects scans the Cursor state database for workspaces that
// have at least one composer session with recorded file edits. Results are
// sorted by most-recent activity first.
func (app *App) BrowseCursorProjects() ([]cursor.ScannedProject, error) {
	return cursor.BrowseProjects()
}

// OnCursorDatabaseChanged is called by the Cursor driver whenever the
// state.vscdb file changes. It triggers incremental processing for every
// active project that has a Cursor source.
//
// This method satisfies the cursor.ChangeHandler interface.
// Guidelines: Don't fire-and-forget goroutines — track with WaitGroup (go-style-guide.md)
func (app *App) OnCursorDatabaseChanged() {
	projects, err := app.GetProjects()
	if err != nil {
		logWarningf(app.logger, "Cursor change handler: failed to load projects: %v", err)
		return
	}

	cursorDriver, ok := app.drivers[AgentSourceCursor]
	if !ok {
		return
	}

	for _, project := range projects {
		if !project.Active {
			continue
		}

		// Only process Cursor sources — other agents (VS Code, Claude Code)
		// have their own watchers and should not be triggered here.
		var cursorSource *AgentSource
		for i := range project.Sources {
			if project.Sources[i].Type == AgentSourceCursor {
				cursorSource = &project.Sources[i]
				break
			}
		}
		if cursorSource == nil {
			continue
		}

		app.waitGroup.Add(1)
		go func(p Project, source AgentSource) {
			defer app.waitGroup.Done()
			// Notify the frontend so it can refresh the chat file list even
			// when no new files are written (e.g. new empty chat, mid-stream).
			app.emitter.Emit("cursor:changed", map[string]string{"projectId": p.ID})

			watchDir := effectiveWatchDir(source, p)
			if watchDir == "" {
				return
			}

			count, err := cursorDriver.ProcessAll(watchDir, p.OutputDir, app.makeProcessCallbacks(p.ID))
			if err != nil {
				logWarningf(app.logger, "Cursor: processing error for %s: %v", p.Name, err)
			} else if count > 0 {
				logInfof(app.logger, "Cursor: processed %d session(s) for %s", count, p.Name)
				if err := app.updateLastProcessed(p.ID); err != nil {
					logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
				}
			}
		}(project, *cursorSource)
	}
}

// ProcessClaudeCodeSessions processes all .jsonl session files for a Claude Code
// project. This is the "Process All Now" path — it scans the transcript directory,
// parses each session, and writes contrail markdown files.
// Delegates to the Claude Code driver's ProcessAll.
func (app *App) ProcessClaudeCodeSessions(projectID, transcriptDirectory, outputDirectory string) (int, error) {
	driver, ok := app.drivers[AgentSourceClaudeCode]
	if !ok {
		return 0, fmt.Errorf("no Claude Code driver registered")
	}

	count, err := driver.ProcessAll(transcriptDirectory, outputDirectory, app.makeProcessCallbacks(projectID))
	if err != nil {
		return 0, err
	}

	// Update lastProcessedAt
	if count > 0 {
		if err := app.updateLastProcessed(projectID); err != nil {
			logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
		}
		app.analytics.TrackProcessAll("claudecode", count)
	}

	return count, nil
}

// ProcessAllSessions iterates every source attached to a project and
// delegates to the corresponding driver's ProcessAll. This is the
// generic "Process All Now" entry-point — callers don't need to know
// which agents are configured.
func (app *App) ProcessAllSessions(projectID string) (int, error) {
	projects, err := app.GetProjects()
	if err != nil {
		return 0, fmt.Errorf("failed to load projects: %w", err)
	}

	var project *Project
	for i := range projects {
		if projects[i].ID == projectID {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		return 0, fmt.Errorf("project %s not found", projectID)
	}

	totalCount := 0
	callbacks := app.makeProcessCallbacks(projectID)

	for _, source := range project.Sources {
		driver, ok := app.drivers[source.Type]
		if !ok {
			logWarningf(app.logger, "No driver registered for %s, skipping", source.Type)
			continue
		}

		watchDir := effectiveWatchDir(source, *project)
		if watchDir == "" {
			continue
		}

		count, err := driver.ProcessAll(watchDir, project.OutputDir, callbacks)
		if err != nil {
			logWarningf(app.logger, "ProcessAll failed for %s source: %v", source.Type, err)
			continue
		}
		totalCount += count
	}

	if totalCount > 0 {
		if err := app.updateLastProcessed(projectID); err != nil {
			logWarningf(app.logger, "Failed to update lastProcessed: %v", err)
		}
	}

	return totalCount, nil
}

// --- Driver Helpers ---

// hasSource checks whether a project has an agent source of the given type.
func hasSource(project Project, sourceType AgentSourceType) bool {
	for _, source := range project.Sources {
		if source.Type == sourceType {
			return true
		}
	}
	return false
}

// effectiveWatchDir returns the watch directory for a source, falling back
// to the project-level WatchDir for backward compatibility with older projects.
func effectiveWatchDir(source AgentSource, project Project) string {
	if source.WatchDir != "" {
		return source.WatchDir
	}
	return project.WatchDir
}

// activateProjectSources activates all agent drivers for the given project.
func (app *App) activateProjectSources(project Project) {
	for _, source := range project.Sources {
		if driver, ok := app.drivers[source.Type]; ok {
			watchDir := effectiveWatchDir(source, project)
			if err := driver.Activate(project.ID, project.WorkspacePath, watchDir); err != nil {
				logWarningf(app.logger, "Failed to activate %s for %s: %v", source.Type, project.Name, err)
			}
		}
	}
	// Legacy: projects without Sources but with WatchDir
	if len(project.Sources) == 0 && project.WatchDir != "" {
		if driver, ok := app.drivers[AgentSourceVSCode]; ok {
			if err := driver.Activate(project.ID, project.WorkspacePath, project.WatchDir); err != nil {
				logWarningf(app.logger, "Failed to activate watcher for %s: %v", project.Name, err)
			}
		}
	}
}

// deactivateProjectSources deactivates all agent drivers for the given project.
func (app *App) deactivateProjectSources(project Project) {
	for _, source := range project.Sources {
		if driver, ok := app.drivers[source.Type]; ok {
			watchDir := effectiveWatchDir(source, project)
			driver.Deactivate(project.WorkspacePath, watchDir)
		}
	}
	// Legacy: projects without Sources but with WatchDir
	if len(project.Sources) == 0 && project.WatchDir != "" {
		if driver, ok := app.drivers[AgentSourceVSCode]; ok {
			driver.Deactivate(project.WorkspacePath, project.WatchDir)
		}
	}
}

// teardownProjectSources calls Teardown on all agent drivers for the given project.
func (app *App) teardownProjectSources(project Project) {
	for _, source := range project.Sources {
		if driver, ok := app.drivers[source.Type]; ok {
			if err := driver.Teardown(project.WorkspacePath); err != nil {
				logWarningf(app.logger, "Failed to teardown %s for %s: %v", source.Type, project.Name, err)
			}
		}
	}
}
