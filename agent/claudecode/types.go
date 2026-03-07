package claudecode

// SignalFile represents the JSON payload written by the Claude Code Stop hook.
type SignalFile struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	PermissionMode       string `json:"permission_mode,omitempty"`
	HookEventName        string `json:"hook_event_name,omitempty"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// toolUseResult holds stdout/stderr from a tool execution.
type toolUseResult struct {
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

// contentBlock is a single block within an assistant message's content array.
// It can be a text block, a thinking block, a tool_use block, or a tool_result block.
type contentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Thinking  string                 `json:"thinking,omitempty"`
	Signature string                 `json:"signature,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   interface{}            `json:"content,omitempty"` // string or nested blocks
	IsError   bool                   `json:"is_error,omitempty"`
}

// messageUsage tracks token consumption.
type messageUsage struct {
	InputTokens              int `json:"input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// lineMessage is the message payload within a JSONL line.
type lineMessage struct {
	Role       string         `json:"role"`
	Model      string         `json:"model,omitempty"`
	ID         string         `json:"id,omitempty"`
	Content    interface{}    `json:"content"` // string or []contentBlock
	StopReason *string        `json:"stop_reason,omitempty"`
	Usage      *messageUsage  `json:"usage,omitempty"`
}

// --- Claude Code JSONL line structures ---

// jsonlLine represents a single line in a Claude Code .jsonl session file.
type jsonlLine struct {
	Type      string       `json:"type"`
	UUID      string       `json:"uuid,omitempty"`
	ParentUUID string      `json:"parentUuid,omitempty"`
	SessionID string       `json:"sessionId,omitempty"`
	Timestamp string       `json:"timestamp,omitempty"`
	Cwd       string       `json:"cwd,omitempty"`
	IsMeta    bool         `json:"isMeta,omitempty"`
	IsSidechain bool       `json:"isSidechain,omitempty"`
	UserType  string       `json:"userType,omitempty"`
	Message   *lineMessage `json:"message,omitempty"`
	RequestID string       `json:"requestId,omitempty"`

	// Tool result fields
	ToolUseResult          *toolUseResult `json:"toolUseResult,omitempty"`
	SourceToolAssistantUUID string        `json:"sourceToolAssistantUUID,omitempty"`

	// queue-operation fields
	Operation string `json:"operation,omitempty"`

	// file-history-snapshot fields
	MessageID string      `json:"messageId,omitempty"`
	Snapshot  interface{} `json:"snapshot,omitempty"`
}
