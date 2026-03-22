//go:build windows

package claudecode

// hookCommand is the shell command written into .claude/settings.local.json.
// On Windows, Claude Code hooks run via the system shell. We use PowerShell to
// pipe the hook's stdin JSON into a timestamped signal file under %USERPROFILE%.
var hookCommand = `powershell -NoProfile -Command "[Console]::In.ReadToEnd() | Set-Content -Encoding utf8 ($env:USERPROFILE + '\contrails\hook-signals\' + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + '_' + $PID + '.json')"`
