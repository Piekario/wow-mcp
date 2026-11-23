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

// report is the journal: every session of the history (oldest first), the latest ones to describe and the records.
type report struct {
	All     []Session
	Shown   []Session
	Records Records
}

func build(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (report, error) {
	n := q.Sessions
	switch {
	case n == 0:
		n = defaultSessions
	case n < 0 || n > maxSessions:
		return report{}, fmt.Errorf("sessions must be from 1 to %d (default %d), got %d", maxSessions, defaultSessions, q.Sessions)
	}
	if tr.History == nil {
		return report{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the journal is built from it")
	}
	stretches, err := tr.Sessions(name, current, 0)
	if err != nil {
		return report{}, err
	}
	all := make([]Session, len(stretches))
	for i, st := range stretches {
		all[i] = Describe(st)
	}
	return report{All: all, Shown: all[max(len(all)-n, 0):], Records: records(all, stretches)}, nil
}
