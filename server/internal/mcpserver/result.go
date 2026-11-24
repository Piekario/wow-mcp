// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult packs any value as readable JSON in a text response.
// When the snapshot is stale, a separate warning line precedes the JSON so it cannot be overlooked.
func textResult(value any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	content := []mcp.Content{}
	if warning := staleWarning(value); warning != "" {
		content = append(content, &mcp.TextContent{Text: "WARNING: " + warning})
	}
	content = append(content, &mcp.TextContent{Text: string(data)})
	return &mcp.CallToolResult{Content: content}, nil, nil
}

// staleWarning extracts the stale-data warning from the response's warning or snapshot.warning field.
func staleWarning(value any) string {
	out, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if warning, ok := out["warning"].(string); ok {
		return warning
	}
	meta, _ := out["snapshot"].(map[string]any)
	warning, _ := meta["warning"].(string)
	return warning
}

// readOnly marks a tool as reading only a local file: it changes nothing, repeating a call gives the same answer
// (until the game saves a new dump), and it never reaches outside the machine.
func readOnly(title string) *mcp.ToolAnnotations {
	closedWorld := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closedWorld}
}

// ownFileWriter marks a tool that writes, but only to the server's own file (not in the game and not elsewhere).
// It is not idempotent: adding the same goal twice adds two goals.
func ownFileWriter(title string) *mcp.ToolAnnotations {
	closedWorld, destructive := false, true // remove deletes a goal from the goals file
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: false, DestructiveHint: &destructive, OpenWorldHint: &closedWorld}
}

// ownFileAdder marks a tool that only adds a file to the server's own directory: nothing is removed, and a
// repeated call writes the same file again.
func ownFileAdder(title string) *mcp.ToolAnnotations {
	closedWorld, destructive := false, false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: false, DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &closedWorld}
}
