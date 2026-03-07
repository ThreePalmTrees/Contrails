package agent

// SessionParser abstracts the parsing of agent session files into a
// common ParsedSession. Both VS Code and Claude Code parsers implement
// this interface, enforcing the Dependency Inversion Principle.
// Style: Verify Interface Compliance (go-style-guide.md)
type SessionParser interface {
	ParseFile(filePath string) (*ParsedSession, error)
}

// --- Parsed output types ---

// MessagePartType identifies the type of a message part.
// These are agent-agnostic content primitives — every type defined here
// must be a concept that multiple agents could plausibly produce.
// Agent-specific semantics belong in the agent's parser, not here.
type MessagePartType string

const (
	PartText       MessagePartType = "text"         // Narrative text
	PartToolCall   MessagePartType = "tool_call"    // Agent invoked a tool
	PartThinking   MessagePartType = "thinking"     // Internal reasoning trace (CoT / extended thinking)
	PartToolResult MessagePartType = "tool_result"  // Output returned from a tool invocation
	PartFileEdit   MessagePartType = "file_edit"    // A file was created or modified
	PartCodeBlock  MessagePartType = "code_block"   // Inline code block
	PartReference  MessagePartType = "reference"    // Reference to a file or symbol
)

// ParsedSession is the agent-agnostic representation of a chat session.
// Both VS Code and Claude Code parsers produce this type.
type ParsedSession struct {
	SessionID     string          `json:"sessionId"`
	Title         string          `json:"title"`
	CreatedAt     string          `json:"createdAt"`
	CreatedAtMs   int64           `json:"createdAtMs"`
	LastMessageAt string          `json:"lastMessageAt"`
	Model         string          `json:"model,omitempty"`
	User          string          `json:"user"`
	Agent         string          `json:"agent"`
	Messages      []ParsedMessage `json:"messages"`
}

// ParsedMessage represents a single message (user or assistant) in a parsed session.
type ParsedMessage struct {
	Timestamp            string        `json:"timestamp"`
	Role                 string        `json:"role"`
	Content              string        `json:"content"`
	Parts                []MessagePart `json:"parts,omitempty"`
	FilesEdited          []string      `json:"filesEdited,omitempty"`
	Model                string        `json:"model,omitempty"`
	Canceled             bool          `json:"canceled,omitempty"`
	Confirmation         string        `json:"confirmation,omitempty"`
	Attachments          []string      `json:"attachments,omitempty"`
	MaxToolCallsExceeded bool          `json:"maxToolCallsExceeded,omitempty"`
}

// MessagePart represents a single element in the interleaved response stream.
type MessagePart struct {
	Type       MessagePartType `json:"type"`
	Content    string          `json:"content,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	ToolArgs   string          `json:"toolArgs,omitempty"`
	ToolDetail *ToolDetail     `json:"toolDetail,omitempty"`
	FilePath   string          `json:"filePath,omitempty"`
	IsEdit     bool            `json:"isEdit,omitempty"`
}

// ToolDetail holds rich, tool-specific data extracted from toolSpecificData
// and supplementary result information (e.g. file paths from search results).
type ToolDetail struct {
	Kind        string     `json:"kind"`                  // "terminal", "todoList", etc.
	Command     string     `json:"command,omitempty"`     // terminal: the command that was run
	Todos       []TodoItem `json:"todos,omitempty"`       // todoList: the todo items
	ResultFiles []string   `json:"resultFiles,omitempty"` // file paths returned by search tools
}

// TodoItem represents a single item in a todo list.
type TodoItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // "not-started", "in-progress", "completed"
}
