//go:build linux

package claudecode

// hookCommand is the shell command written into .claude/settings.local.json.
// It pipes the hook's stdin JSON into a timestamped signal file.
// Uses epoch seconds + PID for the filename — identical to macOS since Linux
// has the same standard date and cat commands.
var hookCommand = "cat > ~/contrails/hook-signals/$(date +%s)_$$.json"
