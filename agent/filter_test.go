package agent

import (
	"testing"
)

// helpers ---------------------------------------------------------------

func makeSession(parts ...MessagePart) *ParsedSession {
	return &ParsedSession{
		Messages: []ParsedMessage{
			{
				Role:        "assistant",
				FilesEdited: []string{"a.go", "b.go"},
				Parts:       parts,
			},
		},
	}
}

func countParts(session *ParsedSession, typ MessagePartType) int {
	n := 0
	for _, msg := range session.Messages {
		for _, p := range msg.Parts {
			if p.Type == typ {
				n++
			}
		}
	}
	return n
}

// DefaultContrailFilters ------------------------------------------------

func TestDefaultContrailFilters_AllEnabled(t *testing.T) {
	f := DefaultContrailFilters()
	if !f.Thinking || !f.ToolCalls || !f.SubagentContent {
		t.Errorf("expected all defaults true, got %+v", f)
	}
}

// ApplyContrailFilters — fast path --------------------------------------

func TestApplyContrailFilters_AllEnabled_NoOp(t *testing.T) {
	session := makeSession(
		MessagePart{Type: PartThinking, Content: "think"},
		MessagePart{Type: PartToolCall, Tool: "Bash"},
		MessagePart{Type: PartText, Content: "done"},
	)
	ApplyContrailFilters(session, DefaultContrailFilters())
	if len(session.Messages[0].Parts) != 3 {
		t.Errorf("expected 3 parts untouched, got %d", len(session.Messages[0].Parts))
	}
}

func TestApplyContrailFilters_NilSession_NoPanic(t *testing.T) {
	ApplyContrailFilters(nil, DefaultContrailFilters())
}

// ApplyContrailFilters — thinking ---------------------------------------

func TestApplyContrailFilters_ThinkingFalse_StripsThinking(t *testing.T) {
	session := makeSession(
		MessagePart{Type: PartThinking, Content: "internal thoughts"},
		MessagePart{Type: PartText, Content: "answer"},
		MessagePart{Type: PartToolCall, Tool: "Read"},
	)
	ApplyContrailFilters(session, ContrailFilters{Thinking: false, ToolCalls: true, SubagentContent: true})

	if countParts(session, PartThinking) != 0 {
		t.Error("expected thinking parts removed")
	}
	if countParts(session, PartText) != 1 {
		t.Error("expected text parts preserved")
	}
	if countParts(session, PartToolCall) != 1 {
		t.Error("expected tool call parts preserved")
	}
}

func TestApplyContrailFilters_ThinkingTrue_PreservesThinking(t *testing.T) {
	session := makeSession(
		MessagePart{Type: PartThinking, Content: "think"},
		MessagePart{Type: PartText, Content: "answer"},
	)
	ApplyContrailFilters(session, ContrailFilters{Thinking: true, ToolCalls: true, SubagentContent: true})

	if countParts(session, PartThinking) != 1 {
		t.Error("expected thinking part preserved")
	}
}

// ApplyContrailFilters — tool calls -------------------------------------

func TestApplyContrailFilters_ToolCallsFalse_StripsAllToolParts(t *testing.T) {
	session := makeSession(
		MessagePart{Type: PartText, Content: "before"},
		MessagePart{Type: PartToolCall, Tool: "Bash"},
		MessagePart{Type: PartToolResult, Content: "result"},
		MessagePart{Type: PartFileEdit, FilePath: "main.go"},
		MessagePart{Type: PartCodeBlock, FilePath: "snippet.go"},
		MessagePart{Type: PartText, Content: "after"},
	)
	ApplyContrailFilters(session, ContrailFilters{Thinking: true, ToolCalls: false, SubagentContent: true})

	msg := session.Messages[0]
	if len(msg.Parts) != 2 {
		t.Errorf("expected 2 text parts, got %d", len(msg.Parts))
	}
	for _, p := range msg.Parts {
		if p.Type != PartText {
			t.Errorf("expected only PartText parts, got %s", p.Type)
		}
	}
}

func TestApplyContrailFilters_ToolCallsFalse_ClearsFilesEdited(t *testing.T) {
	session := makeSession(MessagePart{Type: PartText, Content: "done"})
	ApplyContrailFilters(session, ContrailFilters{Thinking: true, ToolCalls: false, SubagentContent: true})

	if len(session.Messages[0].FilesEdited) != 0 {
		t.Error("expected FilesEdited cleared when tool calls are off")
	}
}

func TestApplyContrailFilters_ToolCallsTrue_PreservesFilesEdited(t *testing.T) {
	session := makeSession(MessagePart{Type: PartText, Content: "done"})
	ApplyContrailFilters(session, DefaultContrailFilters())

	if len(session.Messages[0].FilesEdited) != 2 {
		t.Error("expected FilesEdited preserved when tool calls are on")
	}
}

// ApplyContrailFilters — sub-agent content ------------------------------

func TestApplyContrailFilters_SubagentFalse_NilsSubParts(t *testing.T) {
	session := makeSession(
		MessagePart{
			Type: PartToolCall,
			Tool: "Agent",
			SubParts: []MessagePart{
				{Type: PartText, Content: "sub output"},
				{Type: PartToolCall, Tool: "Read"},
			},
		},
	)
	ApplyContrailFilters(session, ContrailFilters{Thinking: true, ToolCalls: true, SubagentContent: false})

	parts := session.Messages[0].Parts
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Type != PartToolCall {
		t.Errorf("expected tool call part kept, got %s", parts[0].Type)
	}
	if parts[0].SubParts != nil {
		t.Error("expected SubParts nil when subagent content is off")
	}
}

func TestApplyContrailFilters_SubagentTrue_PreservesSubParts(t *testing.T) {
	subParts := []MessagePart{{Type: PartText, Content: "sub"}}
	session := makeSession(
		MessagePart{Type: PartToolCall, Tool: "Agent", SubParts: subParts},
	)
	ApplyContrailFilters(session, DefaultContrailFilters())

	if len(session.Messages[0].Parts[0].SubParts) != 1 {
		t.Error("expected SubParts preserved when subagent content is on")
	}
}

// ApplyContrailFilters — multi-message sessions -------------------------

func TestApplyContrailFilters_MultipleMessages_FiltersEach(t *testing.T) {
	session := &ParsedSession{
		Messages: []ParsedMessage{
			{Role: "assistant", Parts: []MessagePart{{Type: PartThinking, Content: "t1"}, {Type: PartText, Content: "a1"}}},
			{Role: "assistant", Parts: []MessagePart{{Type: PartThinking, Content: "t2"}, {Type: PartText, Content: "a2"}}},
		},
	}
	ApplyContrailFilters(session, ContrailFilters{Thinking: false, ToolCalls: true, SubagentContent: true})

	for i, msg := range session.Messages {
		if countParts(&ParsedSession{Messages: []ParsedMessage{msg}}, PartThinking) != 0 {
			t.Errorf("message %d: expected thinking stripped", i)
		}
		if countParts(&ParsedSession{Messages: []ParsedMessage{msg}}, PartText) != 1 {
			t.Errorf("message %d: expected text preserved", i)
		}
	}
}
