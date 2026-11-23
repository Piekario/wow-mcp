// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"wowmcp/internal/game/doctor"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// writableChecker is the optional history port used by wow_doctor to verify that the history directory can be written.
type writableChecker interface{ CheckWritable() error }

func (s *service) doctorTool() tool {
	return tool{
		name: "wow_doctor", title: "Diagnose the setup", args: argNames[characterArgs](),
		description: "One-call diagnosis when something looks wrong or missing: an ordered list of checks, each with status ok, warn or fail, a detail and a fix " +
			"(dump file found and which account was chosen, dump age, addon version versus the expected one, section errors from the addon, slow dump, disabled sections, " +
			"window sections never captured and which window to open, unloaded dump from exiting the game, non-English client notes, history writable), " +
			"plus a one-line summary and the nextStep to tell the player. Call it first, together with or instead of wow_status, when a tool returns an error, an empty section or stale data. " +
			"It reads the same file as the other tools and changes nothing in the game.",
		add: func(server *mcp.Server, def *mcp.Tool) {
			mcp.AddTool(server, def, func(ctx context.Context, req *mcp.CallToolRequest, a characterArgs) (*mcp.CallToolResult, any, error) {
				db, err := s.loadDatabase()
				if err != nil {
					return textResult(doctor.Unreadable(err.Error(), s.doctorFacts(nil, "")))
				}
				name, snap, err := db.Lookup(a.Character)
				if err != nil {
					return nil, nil, err
				}
				return textResult(doctor.Diagnose(snap, s.doctorFacts(db, name), s.Freshness))
			})
		},
	}
}

// doctorFacts gathers what the domain cannot read itself: the file and its account, the history directory and the goals file.
func (s *service) doctorFacts(db *snapshot.Database, character string) doctor.Facts {
	f := doctor.Facts{ServerVersion: s.Version, Character: character, HistoryDir: s.HistoryDir, GoalsFile: s.GoalsFile}
	if db != nil {
		f.File, f.FileModified = db.Path, db.ModTime
		f.Account, f.OtherAccounts, f.AccountPinned = db.Account, db.OtherAccounts, db.AccountPinned
	}
	if s.History == nil || s.HistoryDir == "" {
		f.HistoryDir = ""
		return f
	}
	if w, ok := s.History.(writableChecker); ok {
		if err := w.CheckWritable(); err != nil {
			f.HistoryError = err.Error()
		}
	}
	if character != "" {
		if times, err := s.History.List(character); err == nil {
			f.HistoryEntries = len(times)
		}
		if c, ok := s.History.(progress.UnloadedCounter); ok {
			f.HistorySkippedUnloaded = c.SkippedUnloaded(character)
		}
	}
	return f
}
