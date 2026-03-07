# testdata

This directory contains test artifacts used by Go unit tests.

Go's test tooling treats `testdata/` as a special directory:
- It is **ignored by the Go build system** (won't be compiled)
- It is **included in the module** (available during `go test`)
- Tests create temporary files and directories here for filesystem operations

## Structure

```
testdata/
├── fixtures/
│   ├── vscode/            # GitHub Copilot chat session JSON fixtures
│   │   ├── minimal.json           # Smallest valid session (1 request, text only)
│   │   ├── empty_requests.json    # Valid JSON, empty requests array
│   │   ├── with_title.json        # Session with customTitle set
│   │   ├── no_title.json          # Session without customTitle (fallback to ID)
│   │   ├── tool_calls.json        # Session with interleaved text + tool calls
│   │   └── malformed.json         # Invalid JSON for error path testing
│   └── claudecode/         # Claude Code fixtures (coming soon)
└── README.md
```

## Guidelines

- Keep fixture files small and focused on the scenario they test
- Use `t.TempDir()` for tests that create/modify files at runtime
- Never commit generated test output — only committed fixtures
- Organize fixtures by agent type (vscode, claudecode, cursor)
