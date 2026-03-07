package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallHook_CreatesSettingsFile(t *testing.T) {
	projectDirectory := t.TempDir()

	if err := InstallHook(projectDirectory); err != nil {
		t.Fatalf("InstallHook failed: %v", err)
	}

	settingsPath := filepath.Join(projectDirectory, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings file: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	// Verify the hook structure
	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("settings should have 'hooks' key")
	}

	stopHooks, ok := hooks["Stop"].([]interface{})
	if !ok {
		t.Fatal("hooks should have 'Stop' array")
	}

	if len(stopHooks) != 1 {
		t.Fatalf("Expected 1 Stop hook, got %d", len(stopHooks))
	}

	// Verify the hook contains the correct command
	hookWrapper, ok := stopHooks[0].(map[string]interface{})
	if !ok {
		t.Fatal("Stop hook should be an object")
	}

	hooksList, ok := hookWrapper["hooks"].([]interface{})
	if !ok {
		t.Fatal("hook wrapper should have 'hooks' array")
	}

	if len(hooksList) != 1 {
		t.Fatalf("Expected 1 hook entry, got %d", len(hooksList))
	}

	entry, ok := hooksList[0].(map[string]interface{})
	if !ok {
		t.Fatal("hook entry should be an object")
	}

	if entry["type"] != "command" {
		t.Errorf("hook type = %q, want %q", entry["type"], "command")
	}
	if entry["command"] != hookCommand {
		t.Errorf("hook command = %q, want %q", entry["command"], hookCommand)
	}
}

func TestInstallHook_MergesWithExistingSettings(t *testing.T) {
	projectDirectory := t.TempDir()

	// Create existing settings with other content
	claudeDirectory := filepath.Join(projectDirectory, ".claude")
	if err := os.MkdirAll(claudeDirectory, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	existingSettings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"Read", "Write"},
		},
	}
	data, _ := json.MarshalIndent(existingSettings, "", "  ")
	settingsPath := filepath.Join(claudeDirectory, "settings.local.json")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write existing settings: %v", err)
	}

	if err := InstallHook(projectDirectory); err != nil {
		t.Fatalf("InstallHook failed: %v", err)
	}

	// Verify existing settings are preserved
	updatedData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read updated settings: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(updatedData, &settings); err != nil {
		t.Fatalf("Failed to parse updated settings: %v", err)
	}

	// Check permissions still exist
	if _, ok := settings["permissions"]; !ok {
		t.Error("Existing 'permissions' key should be preserved")
	}

	// Check hooks were added
	if _, ok := settings["hooks"]; !ok {
		t.Error("'hooks' key should be added")
	}
}

func TestInstallHook_Idempotent(t *testing.T) {
	projectDirectory := t.TempDir()

	// Install twice
	if err := InstallHook(projectDirectory); err != nil {
		t.Fatalf("First InstallHook failed: %v", err)
	}
	if err := InstallHook(projectDirectory); err != nil {
		t.Fatalf("Second InstallHook failed: %v", err)
	}

	// Should still have exactly 1 hook
	settingsPath := filepath.Join(projectDirectory, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	hooks := settings["hooks"].(map[string]interface{})
	stopHooks := hooks["Stop"].([]interface{})
	if len(stopHooks) != 1 {
		t.Errorf("Expected 1 Stop hook after duplicate install, got %d", len(stopHooks))
	}
}

func TestUninstallHook_RemovesContrailsHook(t *testing.T) {
	projectDirectory := t.TempDir()

	// Install, then uninstall
	if err := InstallHook(projectDirectory); err != nil {
		t.Fatalf("InstallHook failed: %v", err)
	}
	if err := UninstallHook(projectDirectory); err != nil {
		t.Fatalf("UninstallHook failed: %v", err)
	}

	// Verify the hook is removed
	settingsPath := filepath.Join(projectDirectory, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	// Hooks key should be removed (empty)
	if _, ok := settings["hooks"]; ok {
		t.Error("'hooks' key should be removed after uninstall")
	}
}

func TestUninstallHook_NoSettingsFile(t *testing.T) {
	projectDirectory := t.TempDir()

	// Should not error when no settings file exists
	if err := UninstallHook(projectDirectory); err != nil {
		t.Errorf("UninstallHook should not error on missing file: %v", err)
	}
}

func TestReadSignalFile(t *testing.T) {
	temporaryDirectory := t.TempDir()
	signalPath := filepath.Join(temporaryDirectory, "signal.json")

	signalContent := `{
		"session_id": "abc123",
		"transcript_path": "/Users/user/.claude/projects/test/abc123.jsonl",
		"cwd": "/Users/user/projects/test",
		"stop_hook_active": false,
		"last_assistant_message": "Done!"
	}`

	if err := os.WriteFile(signalPath, []byte(signalContent), 0644); err != nil {
		t.Fatalf("Failed to write signal file: %v", err)
	}

	signal, err := ReadSignalFile(signalPath)
	if err != nil {
		t.Fatalf("ReadSignalFile failed: %v", err)
	}

	if signal.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", signal.SessionID, "abc123")
	}
	if signal.Cwd != "/Users/user/projects/test" {
		t.Errorf("Cwd = %q, want %q", signal.Cwd, "/Users/user/projects/test")
	}
	if signal.StopHookActive {
		t.Error("StopHookActive should be false")
	}
}

func TestConsumeSignalFile_DeletesAfterReading(t *testing.T) {
	temporaryDirectory := t.TempDir()
	signalPath := filepath.Join(temporaryDirectory, "signal.json")

	signalContent := `{"session_id": "test", "transcript_path": "/tmp/test.jsonl", "cwd": "/tmp", "stop_hook_active": false}`
	if err := os.WriteFile(signalPath, []byte(signalContent), 0644); err != nil {
		t.Fatalf("Failed to write signal file: %v", err)
	}

	signal, err := ConsumeSignalFile(signalPath)
	if err != nil {
		t.Fatalf("ConsumeSignalFile failed: %v", err)
	}

	if signal.SessionID != "test" {
		t.Errorf("SessionID = %q, want %q", signal.SessionID, "test")
	}

	// File should be deleted
	if _, err := os.Stat(signalPath); !os.IsNotExist(err) {
		t.Error("Signal file should be deleted after consumption")
	}
}
