package main

import (
	"contrails/agent/vscode"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// stubDialogOpener is a test double that returns a fixed path.
type stubDialogOpener struct {
	result string
	err    error
}

func (s *stubDialogOpener) OpenDirectoryDialog(_ wailsRuntime.OpenDialogOptions) (string, error) {
	return s.result, s.err
}

// newTestApp creates an App with an isolated config directory
// so tests never touch the real user config.
func newTestApp(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	app.configDir = t.TempDir()
	// Register a VS Code driver so ProcessChatSessions works in tests.
	app.drivers[AgentSourceVSCode] = vscode.NewDriver(nil, app.logger)
	return app
}

// newTestAppWithRecorders creates an App with recording logger/emitter
// for asserting on log output and event emissions.
func newTestAppWithRecorders(t *testing.T) (*App, *RecordingLogger, *RecordingEmitter) {
	t.Helper()
	app := NewApp()
	app.configDir = t.TempDir()
	logger := &RecordingLogger{}
	emitter := &RecordingEmitter{}
	app.logger = logger
	app.emitter = emitter
	// Register a VS Code driver with the recording logger.
	app.drivers[AgentSourceVSCode] = vscode.NewDriver(nil, logger)
	return app, logger, emitter
}

// --- extractSessionID ---

func TestExtractSessionID_ValidMarker(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "standard header",
			header:   "# My Session\n\n- **Session ID:** `abc-123-def`\n- **Created:** 2024-01-01",
			expected: "abc-123-def",
		},
		{
			name:     "uuid format",
			header:   "- **Session ID:** `550e8400-e29b-41d4-a716-446655440000`\n",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "at start of string",
			header:   "**Session ID:** `myid`",
			expected: "myid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionID(tt.header)
			if got != tt.expected {
				t.Errorf("extractSessionID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractSessionID_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"empty string", ""},
		{"no marker", "# Title\n\nSome content"},
		{"incomplete marker - no closing backtick", "**Session ID:** `no-closing-backtick"},
		{"wrong format", "Session ID: abc-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionID(tt.header)
			if got != "" {
				t.Errorf("extractSessionID() = %q, want empty string", got)
			}
		})
	}
}

// --- extractLastMessageDate ---

func TestExtractLastMessageDate_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")

	var ts int64 = 1708000060000
	os.WriteFile(filePath, []byte(`{"kind":0,"v":{"creationDate":1708000060000,"sessionId":"test-id","requests":[]}}`+"\n"), 0644)

	got, err := vscode.ExtractLastMessageDate(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ts {
		t.Errorf("ExtractLastMessageDate() = %d, want %d", got, ts)
	}
}

func TestExtractLastMessageDate_MissingField(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "session.jsonl")

	os.WriteFile(filePath, []byte(`{"kind":0,"v":{"sessionId":"abc","requests":[]}}`+"\n"), 0644)

	got, err := vscode.ExtractLastMessageDate(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("ExtractLastMessageDate() = %d, want 0", got)
	}
}

func TestExtractLastMessageDate_FileNotFound(t *testing.T) {
	_, err := vscode.ExtractLastMessageDate("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtractLastMessageDate_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad.jsonl")
	os.WriteFile(filePath, []byte(`{not json`), 0644)

	got, err := vscode.ExtractLastMessageDate(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 for invalid JSONL, got %d", got)
	}
}

// --- NewApp ---

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp() returned nil")
	}
	if app.lastFileHashes == nil {
		t.Error("lastFileHashes map not initialized")
	}
	if app.logger == nil {
		t.Error("logger not initialized")
	}
	if app.emitter == nil {
		t.Error("emitter not initialized")
	}
}

// --- GetDefaultOutputDir ---

func TestGetDefaultOutputDir(t *testing.T) {
	app := NewApp()
	tests := []struct {
		name      string
		workspace string
		expected  string
	}{
		{"normal path", "/Users/test/my-project", "/Users/test/my-project/contrails"},
		{"empty workspace", "", ""},
		{"root path", "/", "/contrails"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.GetDefaultOutputDir(tt.workspace)
			if got != tt.expected {
				t.Errorf("GetDefaultOutputDir(%q) = %q, want %q", tt.workspace, got, tt.expected)
			}
		})
	}

	t.Run("workspace file without folders", func(t *testing.T) {
		dir := t.TempDir()
		wsFile := filepath.Join(dir, "myproject.code-workspace")
		os.WriteFile(wsFile, []byte("{}"), 0644)
		got := app.GetDefaultOutputDir(wsFile)
		expected := filepath.Join(dir, "contrails")
		if got != expected {
			t.Errorf("GetDefaultOutputDir(%q) = %q, want %q", wsFile, got, expected)
		}
	})

	t.Run("workspace file with folders uses first folder", func(t *testing.T) {
		dir := t.TempDir()
		wsFile := filepath.Join(dir, "myproject.code-workspace")
		content := `{"folders":[{"path":"/Users/test/project-frontend"},{"path":"/Users/test/project-backend"}]}`
		os.WriteFile(wsFile, []byte(content), 0644)
		got := app.GetDefaultOutputDir(wsFile)
		expected := "/Users/test/project-frontend/contrails"
		if got != expected {
			t.Errorf("GetDefaultOutputDir(%q) = %q, want %q", wsFile, got, expected)
		}
	})

	t.Run("workspace file with relative folder path", func(t *testing.T) {
		dir := t.TempDir()
		wsFile := filepath.Join(dir, "myproject.code-workspace")
		content := `{"folders":[{"path":"src/frontend"}]}`
		os.WriteFile(wsFile, []byte(content), 0644)
		got := app.GetDefaultOutputDir(wsFile)
		expected := filepath.Join(dir, "src", "frontend", "contrails")
		if got != expected {
			t.Errorf("GetDefaultOutputDir(%q) = %q, want %q", wsFile, got, expected)
		}
	})
}

// --- ValidateOutputDir ---

func TestValidateOutputDir_WritableDir(t *testing.T) {
	app := NewApp()
	dir := t.TempDir()

	result := app.ValidateOutputDir(dir)
	if result != "" {
		t.Errorf("expected empty string for writable dir, got %q", result)
	}
}

func TestValidateOutputDir_NonexistentCreatable(t *testing.T) {
	app := NewApp()
	dir := filepath.Join(t.TempDir(), "newsubdir")

	result := app.ValidateOutputDir(dir)
	if result != "" {
		t.Errorf("expected empty string for creatable dir, got %q", result)
	}

	// Should have cleaned up
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("ValidateOutputDir should clean up the test directory")
	}
}

func TestValidateOutputDir_PathIsFile(t *testing.T) {
	app := NewApp()
	tmpFile := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(tmpFile, []byte("hi"), 0644)

	result := app.ValidateOutputDir(tmpFile)
	if result != "Path exists but is not a directory" {
		t.Errorf("expected 'Path exists but is not a directory', got %q", result)
	}
}

func TestValidateOutputDir_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping read-only test as root")
	}

	app := NewApp()
	dir := t.TempDir()
	os.Chmod(dir, 0444)
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	result := app.ValidateOutputDir(dir)
	if result != "Directory is not writable" {
		t.Errorf("expected 'Directory is not writable', got %q", result)
	}
}

// --- GetWorkspaceStoragePath ---

func TestGetWorkspaceStoragePath(t *testing.T) {
	app := NewApp()
	path := app.GetWorkspaceStoragePath()

	if !strings.Contains(path, "workspaceStorage") {
		t.Errorf("expected path to contain 'workspaceStorage', got %q", path)
	}
}

// --- processFileReturn ---

func TestProcessFileReturn_ValidSession(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, duration, err := app.processFileReturn("testdata/fixtures/vscode/minimal.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result path")
	}
	if duration == 0 {
		t.Error("expected non-zero duration")
	}

	// Verify the output file was created
	if _, err := os.Stat(result); os.IsNotExist(err) {
		t.Errorf("output file not created: %s", result)
	}
}

func TestProcessFileReturn_EmptySession(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, duration, err := app.processFileReturn("testdata/fixtures/vscode/empty_requests.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for empty session, got %q", result)
	}
	if duration == 0 {
		t.Error("expected non-zero duration even for skipped session")
	}
}

func TestProcessFileReturn_InvalidJSON(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	_, _, err := app.processFileReturn("testdata/fixtures/vscode/malformed.jsonl", outputDir)
	if err == nil {
		t.Error("expected error for invalid JSONL")
	}
}

func TestProcessFileReturn_FileNotFound(t *testing.T) {
	app := NewApp()
	_, _, err := app.processFileReturn("/nonexistent/file.jsonl", t.TempDir())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestProcessFileReturn_TitledSession(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, _, err := app.processFileReturn("testdata/fixtures/vscode/with_title.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output filename should contain the custom title
	if !strings.Contains(filepath.Base(result), "My Custom JSONL Title") {
		t.Errorf("expected filename to contain title, got %q", filepath.Base(result))
	}
}

func TestProcessFileReturn_UntitledSession(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, _, err := app.processFileReturn("testdata/fixtures/vscode/no_title.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output filename should fall back to session ID
	if !strings.Contains(filepath.Base(result), "jsonl-no-title-111") {
		t.Errorf("expected filename to contain session ID, got %q", filepath.Base(result))
	}
}

func TestProcessFileReturn_ToolCalls(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, _, err := app.processFileReturn("testdata/fixtures/vscode/tool_calls.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	md := string(content)
	// Should contain both text and tool calls in order
	if !strings.Contains(md, "Let me read that file for you") {
		t.Error("expected text before tool call")
	}
	if !strings.Contains(md, "Tool Calls") {
		t.Error("expected tool calls section")
	}
}

// --- projectsFilePath ---

func TestProjectsFilePath_ReturnsCorrectPath(t *testing.T) {
	app := newTestApp(t)
	path, err := app.projectsFilePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("contrails", "projects.json")) {
		t.Errorf("expected path ending in contrails/projects.json, got %q", path)
	}
}

func TestProjectsFilePath_CreatesConfigDir(t *testing.T) {
	app := newTestApp(t)
	path, err := app.projectsFilePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("config path is not a directory")
	}
}

// --- GetProjects / SaveProjects ---

func TestGetProjects_EmptyOnFirstCall(t *testing.T) {
	app := newTestApp(t)
	projects, err := app.GetProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestSaveAndGetProjects_RoundTrip(t *testing.T) {
	app := newTestApp(t)
	input := []Project{
		{ID: "p1", Name: "Project One", WatchDir: "/tmp/watch1", OutputDir: "/tmp/out1", Active: true},
		{ID: "p2", Name: "Project Two", WatchDir: "/tmp/watch2", OutputDir: "/tmp/out2", Active: false},
	}
	if err := app.SaveProjects(input); err != nil {
		t.Fatalf("SaveProjects failed: %v", err)
	}

	got, err := app.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}
	if got[0].ID != "p1" || got[0].Name != "Project One" {
		t.Errorf("project 0 mismatch: %+v", got[0])
	}
	if got[1].ID != "p2" || got[1].Active != false {
		t.Errorf("project 1 mismatch: %+v", got[1])
	}
}

func TestGetProjects_InvalidJSON(t *testing.T) {
	app := newTestApp(t)
	// Write invalid JSON to the projects file
	path, _ := app.projectsFilePath()
	os.WriteFile(path, []byte(`{not json`), 0644)

	_, err := app.GetProjects()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- AddProject ---

func TestAddProject_AppendsToList(t *testing.T) {
	app := newTestApp(t)
	err := app.AddProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})
	if err != nil {
		t.Fatalf("AddProject failed: %v", err)
	}

	projects, _ := app.GetProjects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != "p1" {
		t.Errorf("expected ID 'p1', got %q", projects[0].ID)
	}
}

func TestAddProject_SetsActiveAndLastProcessed(t *testing.T) {
	app := newTestApp(t)
	before := time.Now().UnixMilli()
	app.AddProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})

	projects, _ := app.GetProjects()
	if !projects[0].Active {
		t.Error("expected project to be active")
	}
	if projects[0].LastProcessed < before {
		t.Error("expected LastProcessed to be set to now")
	}
}

func TestAddProject_MultipleProjects(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "First", WatchDir: "/tmp/w1", OutputDir: "/tmp/o1"})
	app.AddProject(Project{ID: "p2", Name: "Second", WatchDir: "/tmp/w2", OutputDir: "/tmp/o2"})

	projects, _ := app.GetProjects()
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

// --- UpdateProject ---

func TestUpdateProject_UpdatesFields(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "Original", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})

	err := app.UpdateProject(Project{ID: "p1", Name: "Updated", WatchDir: "/tmp/w", OutputDir: "/tmp/o", Active: true})
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	projects, _ := app.GetProjects()
	if projects[0].Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", projects[0].Name)
	}
}

func TestUpdateProject_NotFound(t *testing.T) {
	app := newTestApp(t)
	err := app.UpdateProject(Project{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
	if !strings.Contains(err.Error(), "project not found") {
		t.Errorf("expected 'project not found' error, got %q", err.Error())
	}
}

func TestUpdateProject_SetsPausedAtOnPause(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})

	before := time.Now().UnixMilli()
	// Pause the project (Active: true → false)
	app.UpdateProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o", Active: false})

	projects, _ := app.GetProjects()
	if projects[0].PausedAt < before {
		t.Error("expected PausedAt to be set on pause")
	}
}

func TestUpdateProject_ClearsPausedAtOnResume(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})
	// Pause
	app.UpdateProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o", Active: false})
	// Resume
	app.UpdateProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o", Active: true})

	projects, _ := app.GetProjects()
	if projects[0].PausedAt != 0 {
		t.Errorf("expected PausedAt to be cleared on resume, got %d", projects[0].PausedAt)
	}
}

// --- RemoveProject ---

func TestRemoveProject_RemovesFromList(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "First", WatchDir: "/tmp/w1", OutputDir: "/tmp/o1"})
	app.AddProject(Project{ID: "p2", Name: "Second", WatchDir: "/tmp/w2", OutputDir: "/tmp/o2"})

	err := app.RemoveProject("p1")
	if err != nil {
		t.Fatalf("RemoveProject failed: %v", err)
	}

	projects, _ := app.GetProjects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != "p2" {
		t.Errorf("expected remaining project to be 'p2', got %q", projects[0].ID)
	}
}

func TestRemoveProject_UnknownID(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "Test", WatchDir: "/tmp/w", OutputDir: "/tmp/o"})

	// Removing unknown ID should not error (just saves the same list)
	err := app.RemoveProject("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projects, _ := app.GetProjects()
	if len(projects) != 1 {
		t.Errorf("expected 1 project (unchanged), got %d", len(projects))
	}
}

// --- updateLastProcessed ---

func TestUpdateLastProcessed_UpdatesCorrectProject(t *testing.T) {
	app := newTestApp(t)
	app.AddProject(Project{ID: "p1", Name: "A", WatchDir: "/tmp/w1", OutputDir: "/tmp/o1"})
	app.AddProject(Project{ID: "p2", Name: "B", WatchDir: "/tmp/w2", OutputDir: "/tmp/o2"})

	before := time.Now().UnixMilli()
	err := app.updateLastProcessed("p2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projects, _ := app.GetProjects()
	if projects[1].LastProcessed < before {
		t.Error("expected p2's LastProcessed to be updated")
	}
	// p1 should be unchanged (set by AddProject, which sets it to ~now, but
	// the updateLastProcessed call for p2 should not touch p1)
}

func TestUpdateLastProcessed_UnknownID_NoError(t *testing.T) {
	app := newTestApp(t)
	err := app.updateLastProcessed("nonexistent")
	if err != nil {
		t.Errorf("expected nil error for unknown ID, got %v", err)
	}
}

// --- Test helpers for processing pipeline ---

// copyFixture copies a fixture file into a new temp directory, returning the dir path.
func copyFixture(t *testing.T, fixturePath, destName string) string {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixturePath, err)
	}
	if err := os.WriteFile(filepath.Join(dir, destName), data, 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return dir
}

// copyFixtureInto copies a fixture file into an existing directory.
func copyFixtureInto(t *testing.T, fixturePath, destDir, destName string) {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixturePath, err)
	}
	if err := os.WriteFile(filepath.Join(destDir, destName), data, 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
}

// --- ProcessChatSessions ---

func TestProcessChatSessions_ProcessesAllFiles(t *testing.T) {
	app, _, emitter := newTestAppWithRecorders(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session1.jsonl")
	copyFixtureInto(t, "testdata/fixtures/vscode/with_title.jsonl", watchDir, "session2.jsonl")

	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})

	count, err := app.ProcessChatSessions("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files processed, got %d", count)
	}

	// Should have emitted progress events
	progressEvents := 0
	for _, ev := range emitter.Events {
		if ev.Name == "processing:progress" {
			progressEvents++
		}
	}
	if progressEvents != 2 {
		t.Errorf("expected 2 progress events, got %d", progressEvents)
	}
}

func TestProcessChatSessions_SkipsMalformedFiles(t *testing.T) {
	app, logger, _ := newTestAppWithRecorders(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "good.jsonl")
	copyFixtureInto(t, "testdata/fixtures/vscode/malformed.jsonl", watchDir, "bad.jsonl")

	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})

	count, err := app.ProcessChatSessions("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 file processed (skipping malformed), got %d", count)
	}

	if len(logger.WarningMessages) == 0 {
		t.Error("expected warning about malformed file")
	}
}

func TestProcessChatSessions_EmptyDir(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})

	count, err := app.ProcessChatSessions("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files processed, got %d", count)
	}
}

func TestProcessChatSessions_NonexistentDir(t *testing.T) {
	app := newTestApp(t)
	_, err := app.ProcessChatSessions("proj-1", "/nonexistent/dir", t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

// --- ProcessFileIfNeeded ---

func TestProcessFileIfNeeded_NewerFile_Processes(t *testing.T) {
	app, _, emitter := newTestAppWithRecorders(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session.jsonl")

	// Add project with LastProcessed well before the fixture's lastMessageDate (1708000060000)
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1700000000000
	app.SaveProjects(projects)

	filePath := filepath.Join(watchDir, "session.jsonl")
	result, err := app.ProcessFileIfNeeded("proj-1", filePath, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected file to be processed (newer than lastProcessed)")
	}

	// Should have emitted file:processed event
	found := false
	for _, ev := range emitter.Events {
		if ev.Name == "file:processed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file:processed event")
	}
}

func TestProcessFileIfNeeded_OlderFile_Skips(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session.jsonl")

	// Set LastProcessed far in the future
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1800000000000
	app.SaveProjects(projects)

	// Also record the current file hash so the hash-change path doesn't trigger
	filePath := filepath.Join(watchDir, "session.jsonl")
	hash, _ := vscode.ExtractSessionSignature(filePath)
	app.lastFileHashes[filePath] = hash

	result, err := app.ProcessFileIfNeeded("proj-1", filePath, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result (file older than lastProcessed), got %q", result)
	}
}

func TestProcessFileIfNeeded_SameDateDifferentHash_Processes(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	filePath := filepath.Join(watchDir, "session.jsonl")
	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session.jsonl")

	// Set lastProcessed to match the fixture's lastMessageDate exactly
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1708000060000
	app.SaveProjects(projects)

	// Record a different previous hash so the hash-change path triggers
	app.lastFileHashes[filePath] = "some-different-hash"

	result, err := app.ProcessFileIfNeeded("proj-1", filePath, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected file to be processed (hash changed)")
	}
}

func TestProcessFileIfNeeded_SameDateSameHash_Skips(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	filePath := filepath.Join(watchDir, "session.jsonl")
	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session.jsonl")

	// Set lastProcessed to match
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1708000060000
	app.SaveProjects(projects)

	// Record the actual file hash
	hash, _ := vscode.ExtractSessionSignature(filePath)
	app.lastFileHashes[filePath] = hash

	result, err := app.ProcessFileIfNeeded("proj-1", filePath, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected skip (same date + same hash), got %q", result)
	}
}

// --- ProcessModifiedSince ---

func TestProcessModifiedSince_ProcessesNewerFiles(t *testing.T) {
	app, _, emitter := newTestAppWithRecorders(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session1.jsonl")
	copyFixtureInto(t, "testdata/fixtures/vscode/with_title.jsonl", watchDir, "session2.jsonl")

	// Set LastProcessed before the fixtures' lastMessageDate
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1700000000000
	app.SaveProjects(projects)

	count, err := app.ProcessModifiedSince("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files processed, got %d", count)
	}

	fileEvents := 0
	for _, ev := range emitter.Events {
		if ev.Name == "file:processed" {
			fileEvents++
		}
	}
	if fileEvents != 2 {
		t.Errorf("expected 2 file:processed events, got %d", fileEvents)
	}
}

func TestProcessModifiedSince_SkipsOlderFiles(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	copyFixtureInto(t, "testdata/fixtures/vscode/minimal.jsonl", watchDir, "session.jsonl")

	// Set lastProcessed after the fixture's lastMessageDate
	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})
	projects, _ := app.GetProjects()
	projects[0].LastProcessed = 1800000000000
	app.SaveProjects(projects)

	count, err := app.ProcessModifiedSince("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files processed, got %d", count)
	}
}

func TestProcessModifiedSince_EmptyDir(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	app.AddProject(Project{ID: "proj-1", Name: "Test", WatchDir: watchDir, OutputDir: outputDir})

	count, err := app.ProcessModifiedSince("proj-1", watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// --- ProcessSingleFile ---

func TestProcessSingleFile_ValidFile(t *testing.T) {
	app := NewApp()
	outputDir := t.TempDir()

	result, err := app.ProcessSingleFile("testdata/fixtures/vscode/minimal.jsonl", "vscode", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestProcessSingleFile_InvalidFile(t *testing.T) {
	app := NewApp()
	_, err := app.ProcessSingleFile("testdata/fixtures/vscode/malformed.jsonl", "vscode", t.TempDir())
	if err == nil {
		t.Error("expected error for malformed file")
	}
}

// --- HandleDeletedFile ---

func TestHandleDeletedFile_FindsAndFlags(t *testing.T) {
	app, logger, _ := newTestAppWithRecorders(t)
	outputDir := t.TempDir()
	sessionID := "abc-123-def"

	mdContent := "# Test Session\n\n- **Session ID:** `" + sessionID + "`\n- **Created:** 2024-01-01\n\nSome content."
	mdPath := filepath.Join(outputDir, "Test Session.md")
	os.WriteFile(mdPath, []byte(mdContent), 0644)

	err := app.HandleDeletedFile(sessionID+".jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(mdPath)
	if !strings.HasPrefix(string(content), "> ⚠️ **This chat session has been deleted") {
		t.Error("expected deleted banner to be prepended")
	}
	if !strings.Contains(string(content), "Some content.") {
		t.Error("expected original content preserved")
	}
	if len(logger.InfoMessages) == 0 {
		t.Error("expected info log about marking session as deleted")
	}
}

func TestHandleDeletedFile_AlreadyFlagged_NoOp(t *testing.T) {
	app := newTestApp(t)
	outputDir := t.TempDir()
	sessionID := "abc-123-def"

	banner := "> ⚠️ **This chat session has been deleted.** The source JSON file is no longer available.\n\n"
	mdContent := banner + "# Test\n\n- **Session ID:** `" + sessionID + "`\n"
	mdPath := filepath.Join(outputDir, "Test.md")
	os.WriteFile(mdPath, []byte(mdContent), 0644)

	err := app.HandleDeletedFile(sessionID+".jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(mdPath)
	bannerCount := strings.Count(string(content), "⚠️ **This chat session has been deleted")
	if bannerCount != 1 {
		t.Errorf("expected exactly 1 banner, got %d", bannerCount)
	}
}

func TestHandleDeletedFile_NoMatchingMd(t *testing.T) {
	app := newTestApp(t)
	outputDir := t.TempDir()

	os.WriteFile(filepath.Join(outputDir, "Other.md"), []byte("# Other\n\n- **Session ID:** `other-id`"), 0644)

	err := app.HandleDeletedFile("not-matching.jsonl", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleDeletedFile_OutputDirNotExist(t *testing.T) {
	app := newTestApp(t)
	err := app.HandleDeletedFile("some.jsonl", "/nonexistent/output")
	if err != nil {
		t.Errorf("expected nil error for nonexistent output dir, got %v", err)
	}
}

// --- HealContrailNames ---

func TestHealContrailNames_HealsSessionIDFilename(t *testing.T) {
	app, logger, _ := newTestAppWithRecorders(t)
	watchDir := t.TempDir()
	outputDir := t.TempDir()

	sessionID := "jsonl-titled-456"
	copyFixtureInto(t, "testdata/fixtures/vscode/with_title.jsonl", watchDir, sessionID+".jsonl")

	// Create an .md file named with the session ID (the "stale" name)
	mdContent := "# Old Title\n\n- **Session ID:** `" + sessionID + "`\n\nContent"
	os.WriteFile(filepath.Join(outputDir, sessionID+".md"), []byte(mdContent), 0644)

	healed, err := app.HealContrailNames(watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if healed != 1 {
		t.Errorf("expected 1 healed, got %d", healed)
	}

	foundHealLog := false
	for _, msg := range logger.InfoMessages {
		if strings.Contains(msg, "Healed contrail") {
			foundHealLog = true
			break
		}
	}
	if !foundHealLog {
		t.Error("expected info log about healing")
	}
}

func TestHealContrailNames_AlreadyTitled_Skips(t *testing.T) {
	app := newTestApp(t)
	outputDir := t.TempDir()

	mdContent := "# Nice Title\n\n- **Session ID:** `abc-123`\n\nContent"
	os.WriteFile(filepath.Join(outputDir, "Nice Title.md"), []byte(mdContent), 0644)

	healed, err := app.HealContrailNames(t.TempDir(), outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if healed != 0 {
		t.Errorf("expected 0 healed, got %d", healed)
	}
}

func TestHealContrailNames_OutputDirNotExist(t *testing.T) {
	app := newTestApp(t)
	healed, err := app.HealContrailNames(t.TempDir(), "/nonexistent/output")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if healed != 0 {
		t.Errorf("expected 0, got %d", healed)
	}
}

func TestHealContrailNames_SourceJsonMissing_Skips(t *testing.T) {
	app := newTestApp(t)
	watchDir := t.TempDir() // empty
	outputDir := t.TempDir()

	sessionID := "missing-source-id"
	mdContent := "# Title\n\n- **Session ID:** `" + sessionID + "`\n"
	os.WriteFile(filepath.Join(outputDir, sessionID+".md"), []byte(mdContent), 0644)

	healed, err := app.HealContrailNames(watchDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if healed != 0 {
		t.Errorf("expected 0, got %d", healed)
	}
}

// --- SelectChatSessionsDir / SelectOutputDir ---

func TestSelectChatSessionsDir_ReturnsDialogResult(t *testing.T) {
	app := newTestApp(t)
	app.dialogOpener = &stubDialogOpener{result: "/Users/test/workspaceStorage/abc/chatSessions"}

	result, err := app.SelectChatSessionsDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/Users/test/workspaceStorage/abc/chatSessions" {
		t.Errorf("expected dialog result, got %q", result)
	}
}

func TestSelectOutputDir_ReturnsDialogResult(t *testing.T) {
	app := newTestApp(t)
	app.dialogOpener = &stubDialogOpener{result: "/Users/test/output"}

	result, err := app.SelectOutputDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/Users/test/output" {
		t.Errorf("expected dialog result, got %q", result)
	}
}