//go:build darwin

package claudecode

// hookCommand is the shell command written into .claude/settings.local.json.
// It pipes the hook's stdin JSON into a timestamped signal file.
// Uses epoch seconds + PID for the filename — $$ is unique per hook invocation
// and works on all macOS versions (BSD date doesn't support %N on older macOS).
var hookCommand = "cat > ~/contrails/hook-signals/$(date +%s)_$$.json"
