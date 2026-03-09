package main

import "contrails/agent"

// --- Type aliases for backward compatibility ---
// These allow existing code in the main package to continue using
// the unqualified type names while the canonical definitions live
// in the agent package (Dependency Inversion Principle).

type SessionParser = agent.SessionParser
type ParsedSession = agent.ParsedSession
type ParsedMessage = agent.ParsedMessage
type MessagePart = agent.MessagePart
type MessagePartType = agent.MessagePartType
type ToolDetail = agent.ToolDetail
type TodoItem = agent.TodoItem

// Re-export constants so existing references in main still compile.
const (
	PartText      = agent.PartText
	PartToolCall  = agent.PartToolCall
	PartFileEdit  = agent.PartFileEdit
	PartCodeBlock = agent.PartCodeBlock
	PartReference = agent.PartReference
)

// AgentSourceType identifies which agent produced a session.
type AgentSourceType string

const (
	AgentSourceVSCode     AgentSourceType = "vscode"
	AgentSourceClaudeCode AgentSourceType = "claudecode"
	AgentSourceCursor     AgentSourceType = "cursor"
)

// AgentSource describes one agent data source attached to a project.
type AgentSource struct {
	Type     AgentSourceType `json:"type"`               // "vscode" | "claudecode" | "cursor"
	WatchDir string          `json:"watchDir,omitempty"` // VS Code: chatSessions/ dir; Claude Code: empty (signal-based)
}

// Project represents a watched workspace project
type Project struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	WatchDir      string            `json:"watchDir"`                // kept for backward compat with existing VS Code projects
	OutputDir     string            `json:"outputDir"`
	Active        bool              `json:"active"`
	WorkspacePath string            `json:"workspacePath,omitempty"`
	Sources       []AgentSource     `json:"sources,omitempty"`       // one or both agent sources
	LastProcessed int64             `json:"lastProcessed,omitempty"`
	PausedAt      int64             `json:"pausedAt,omitempty"`
	IgnoredChats  map[string]string `json:"ignoredChats,omitempty"`  // filePath → title (for display when source unavailable)
}

// ProcessingProgress reports progress during batch processing
type ProcessingProgress struct {
	ProjectID string `json:"projectId"`
	Current   int    `json:"current"`
	Total     int    `json:"total"`
}

// AppError represents an error to display in the UI
type AppError struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	Message     string `json:"message"`
}

// WatcherEvent is emitted to the frontend
type WatcherEvent struct {
	ProjectID string `json:"projectId"`
	FileName  string `json:"fileName"`
	EventType string `json:"eventType"` // "created", "modified", "removed"
}

// FileProcessedEvent is emitted when a file is processed (for badge tracking)
type FileProcessedEvent struct {
	ProjectID string `json:"projectId"`
	FileName  string `json:"fileName"`
}

// ChatFileInfo describes a single source chat session file and whether
// it has already been parsed into a contrail markdown file.
type ChatFileInfo struct {
	FileName      string `json:"fileName"`
	FilePath      string `json:"filePath"`
	SourceType    string `json:"sourceType"` // "vscode" | "claudecode" | "cursor"
	Parsed          bool   `json:"parsed"`
	PartiallyParsed bool   `json:"partiallyParsed"`
	Title           string `json:"title"`
	LastMessageAt   string `json:"lastMessageAt"`
	ProcessedAt     int64  `json:"processedAt"` // unix ms of the output .md file's mtime; 0 if not parsed
	CreatedAt       int64  `json:"createdAt"`   // unix ms of the first message / session creation
	Ignored         bool   `json:"ignored"`
}
