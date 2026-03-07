package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookCommand is the shell command written into .claude/settings.local.json.
// It pipes the hook's stdin JSON into a timestamped signal file.
const hookCommand = "cat > ~/contrails/hook-signals/$(date +%s%N).json"

// hookEntry represents a single hook command entry.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookWrapper wraps a list of hook entries.
type hookWrapper struct {
	Hooks []hookEntry `json:"hooks"`
}

// InstallHook writes the Claude Code Stop hook into the project's
// .claude/settings.local.json file. It merges with any existing
// settings, preserving other hooks. If the contrails hook is already
// present, no changes are made.
func InstallHook(projectPath string) error {
	settingsDirectory := filepath.Join(projectPath, ".claude")
	if err := os.MkdirAll(settingsDirectory, 0755); err != nil {
		return fmt.Errorf("creating .claude directory: %w", err)
	}

	settingsPath := filepath.Join(settingsDirectory, "settings.local.json")

	// Read existing settings or start with an empty object
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading settings file: %w", err)
		}
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing settings file: %w", err)
		}
	}

	// Check if the contrails hook already exists
	if hasContrailsHook(settings) {
		return nil // Already installed
	}

	// Build the hook entry
	newHook := hookWrapper{
		Hooks: []hookEntry{
			{
				Type:    "command",
				Command: hookCommand,
			},
		},
	}

	// Add to hooks.Stop array
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	stopHooks, _ := hooks["Stop"].([]interface{})

	// Convert the new hook to a generic interface{} via marshal/unmarshal
	hookData, err := json.Marshal(newHook)
	if err != nil {
		return fmt.Errorf("marshaling hook entry: %w", err)
	}
	var hookInterface interface{}
	if err := json.Unmarshal(hookData, &hookInterface); err != nil {
		return fmt.Errorf("unmarshaling hook entry: %w", err)
	}

	stopHooks = append(stopHooks, hookInterface)
	hooks["Stop"] = stopHooks
	settings["hooks"] = hooks

	// Write back
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("writing settings file: %w", err)
	}

	return nil
}

// UninstallHook removes the contrails Stop hook from the project's
// .claude/settings.local.json file, if present.
func UninstallHook(projectPath string) error {
	settingsPath := filepath.Join(projectPath, ".claude", "settings.local.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to uninstall
		}
		return fmt.Errorf("reading settings file: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parsing settings file: %w", err)
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}

	stopHooks, _ := hooks["Stop"].([]interface{})
	if len(stopHooks) == 0 {
		return nil
	}

	// Filter out the contrails hook
	var filtered []interface{}
	for _, hook := range stopHooks {
		if !isContrailsHookEntry(hook) {
			filtered = append(filtered, hook)
		}
	}

	if len(filtered) == len(stopHooks) {
		return nil // Nothing was removed
	}

	if len(filtered) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = filtered
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	return os.WriteFile(settingsPath, output, 0644)
}

// hasContrailsHook checks whether the contrails hook command is already
// present in the settings.
func hasContrailsHook(settings map[string]interface{}) bool {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return false
	}

	stopHooks, _ := hooks["Stop"].([]interface{})
	for _, hook := range stopHooks {
		if isContrailsHookEntry(hook) {
			return true
		}
	}
	return false
}

// isContrailsHookEntry checks if a Stop hook entry contains the contrails command.
func isContrailsHookEntry(hook interface{}) bool {
	hookMap, ok := hook.(map[string]interface{})
	if !ok {
		return false
	}

	hooksList, _ := hookMap["hooks"].([]interface{})
	for _, entry := range hooksList {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if command, ok := entryMap["command"].(string); ok {
			if command == hookCommand {
				return true
			}
		}
	}
	return false
}

// EnsureSignalDirectory creates the signal directory if it doesn't exist.
func EnsureSignalDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	signalDirectory := filepath.Join(home, "contrails", "hook-signals")
	if err := os.MkdirAll(signalDirectory, 0755); err != nil {
		return "", fmt.Errorf("creating signal directory: %w", err)
	}

	return signalDirectory, nil
}

// SignalDirectory returns the path to the hook signal directory.
func SignalDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "contrails", "hook-signals")
}

// ReadSignalFile reads and parses a signal file written by the Claude Code Stop hook.
func ReadSignalFile(filePath string) (*SignalFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading signal file: %w", err)
	}

	var signal SignalFile
	if err := json.Unmarshal(data, &signal); err != nil {
		return nil, fmt.Errorf("parsing signal file: %w", err)
	}

	return &signal, nil
}

// ConsumeSignalFile reads, parses, and then deletes the signal file.
func ConsumeSignalFile(filePath string) (*SignalFile, error) {
	signal, err := ReadSignalFile(filePath)
	if err != nil {
		return nil, err
	}

	// Delete the consumed signal file
	if removeErr := os.Remove(filePath); removeErr != nil {
		// Log but don't fail — the signal was successfully read
		return signal, fmt.Errorf("removing signal file (data still valid): %w", removeErr)
	}

	return signal, nil
}
