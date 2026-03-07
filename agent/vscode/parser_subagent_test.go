package vscode

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSubagentToolCallParsing(t *testing.T) {
	parser := &Parser{}
	parsed, err := parser.ParseFile("../../testdata/fixtures/vscode/subagent_tool_calls_with_rounds.json")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	b, _ := json.MarshalIndent(parsed, "", "  ")
	fmt.Println(string(b))
}
