package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testLogger is a no-op logger for tests.
type testLogger struct{}

func (testLogger) Info(string)    {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

func TestHookEnforcer_RestoresDeletedHook(t *testing.T) {
	// Create a temporary workspace with a .claude directory
	workspacePath := t.TempDir()
	claudeDir := filepath.Join(workspacePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Install the hook initially
	if err := InstallHook(workspacePath); err != nil {
		t.Fatalf("initial InstallHook: %v", err)
	}

	// Verify the hook is installed
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	assertHookInstalled(t, settingsPath)

	// Delete the settings file (simulating user deletion)
	if err := os.Remove(settingsPath); err != nil {
		t.Fatalf("removing settings file: %v", err)
	}

	// Create enforcer, register the path, and run a single enforcement pass
	enforcer := NewHookEnforcer(testLogger{})
	enforcer.Register(workspacePath)
	enforcer.enforce()

	// The hook should be restored
	assertHookInstalled(t, settingsPath)
}

func TestHookEnforcer_RestoresRemovedHookEntry(t *testing.T) {
	// Create a temporary workspace
	workspacePath := t.TempDir()
	claudeDir := filepath.Join(workspacePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Install the hook
	if err := InstallHook(workspacePath); err != nil {
		t.Fatalf("initial InstallHook: %v", err)
	}

	// Overwrite the file with settings that have no hooks but preserve other content
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	otherSettings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{"Bash"},
		},
	}
	data, _ := json.MarshalIndent(otherSettings, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		t.Fatalf("writing settings without hook: %v", err)
	}

	// Run enforcement
	enforcer := NewHookEnforcer(testLogger{})
	enforcer.Register(workspacePath)
	enforcer.enforce()

	// The hook should be restored and permissions preserved
	assertHookInstalled(t, settingsPath)

	// Verify the permissions key is still there
	restored, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(restored, &settings); err != nil {
		t.Fatal(err)
	}
	if _, ok := settings["permissions"]; !ok {
		t.Error("permissions key was lost during enforcement")
	}
}

func TestHookEnforcer_NoOpWhenHookPresent(t *testing.T) {
	workspacePath := t.TempDir()
	claudeDir := filepath.Join(workspacePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Install the hook
	if err := InstallHook(workspacePath); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	infoBefore, _ := os.Stat(settingsPath)
	timeBefore := infoBefore.ModTime()

	// Wait a tiny bit so modtime would differ if the file were rewritten
	time.Sleep(10 * time.Millisecond)

	// Run enforcement — should not rewrite the file
	enforcer := NewHookEnforcer(testLogger{})
	enforcer.Register(workspacePath)
	enforcer.enforce()

	infoAfter, _ := os.Stat(settingsPath)
	timeAfter := infoAfter.ModTime()

	if !timeBefore.Equal(timeAfter) {
		t.Error("enforce() rewrote the file even though the hook was already present")
	}
}

func TestHookEnforcer_UnregisterStopsEnforcement(t *testing.T) {
	workspacePath := t.TempDir()
	claudeDir := filepath.Join(workspacePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	enforcer := NewHookEnforcer(testLogger{})
	enforcer.Register(workspacePath)
	enforcer.Unregister(workspacePath)

	// Delete any existing settings
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	os.Remove(settingsPath)

	// Run enforcement — should do nothing since the path was unregistered
	enforcer.enforce()

	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Error("enforce() created settings file for an unregistered path")
	}
}

func TestHookEnforcer_StartAndStop(t *testing.T) {
	enforcer := NewHookEnforcer(testLogger{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enforcer.Start(ctx)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		enforcer.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds")
	}
}

func TestHookEnforcer_MultipleWorkspaces(t *testing.T) {
	// Create multiple workspace directories
	workspacePaths := make([]string, 3)
	for i := range workspacePaths {
		workspacePaths[i] = t.TempDir()
		claudeDir := filepath.Join(workspacePaths[i], ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	enforcer := NewHookEnforcer(testLogger{})
	for _, path := range workspacePaths {
		enforcer.Register(path)
	}

	enforcer.enforce()

	// All workspaces should have the hook installed
	for _, workspacePath := range workspacePaths {
		settingsPath := filepath.Join(workspacePath, ".claude", "settings.local.json")
		assertHookInstalled(t, settingsPath)
	}
}

// assertHookInstalled verifies the contrails Stop hook is present in the settings file.
func assertHookInstalled(t *testing.T, settingsPath string) {
	t.Helper()

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings file: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parsing settings file: %v", err)
	}

	if !hasContrailsHook(settings) {
		t.Errorf("contrails hook not found in %s", settingsPath)
	}
}
