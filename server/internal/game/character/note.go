// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// Note is the player's note for the assistant (/mcp note in game), e.g. "I want Deadmines today".
// Returns the text and age of the note; nil when there is no note or it is empty.
func Note(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	n := s.Map("note")
	text, _ := n["text"].(string)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	out := map[string]any{"text": text}
	if at, ok := snapshot.AsInt(n["setAt"]); ok {
		out["setAt"] = snapshot.FormatTime(at)
		out["setAgo"] = snapshot.FormatAge(fresh.Age(at)) + " ago"
	}
	return out
}

// NoteText is just the note text ("" when there is none); to be included in prompts as the session goal.
func NoteText(s snapshot.Snapshot) string {
	text, _ := s.Map("note")["text"].(string)
	return strings.TrimSpace(text)
}
