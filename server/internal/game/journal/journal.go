// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package journal is the session journal: the dump history split into play sessions (the same rule as in
// wow_progress) with a description of each one and the character's records. It only reads dumps; Markdown is returned as
// text in the response, and the server never writes a .md file itself.
package journal

import (
	"fmt"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_journal.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Sessions  int    `json:"sessions,omitempty" jsonschema:"how many recent sessions to describe; default 5"`
	Markdown  bool   `json:"markdown,omitempty" jsonschema:"true = additionally a ready Markdown text in the text field (the server does not write .md files itself)"`
}

// The default and maximum number of sessions described.
const (
	defaultSessions = 5
	maxSessions     = 50
)

// View describes the character's latest sessions and its records from the whole history.
func View(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	n := q.Sessions
	switch {
	case n == 0:
		n = defaultSessions
	case n < 0 || n > maxSessions:
		return nil, fmt.Errorf("sessions must be from 1 to %d (default %d), got %d", maxSessions, defaultSessions, q.Sessions)
	}
	if tr.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the journal is built from it")
	}
	stretches, err := tr.Sessions(name, current, 0)
	if err != nil {
		return nil, err
	}
	all := make([]Session, len(stretches))
	for i, st := range stretches {
		all[i] = Describe(st)
	}

	out := map[string]any{
		"totalSessions": len(all),
		"order":         "from the newest session to the oldest",
		"howSessionsAreSplit": "the same rule as wow_progress: a new session after a gap longer than 30 min offline (from /played) or 120 min between dumps; " +
			"the data comes only from dumps (/mcp reload, logout), so what happened between them shows only as differences",
	}
	tr.NoteSkipped(out, name, current)
	shown := all[max(len(all)-n, 0):]
	list := make([]any, 0, len(shown))
	for i := len(shown) - 1; i >= 0; i-- {
		list = append(list, shown[i].View())
	}
	out["sessions"] = list
	if len(all) > len(shown) {
		out["olderSessionsNotShown"] = len(all) - len(shown)
	}

	rec := records(all, stretches)
	if len(rec) > 0 {
		out["records"] = rec
	}
	if q.Markdown {
		out["text"] = markdown(name, current, shown, rec)
	}
	return out, nil
}
