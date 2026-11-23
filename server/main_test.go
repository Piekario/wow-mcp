// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package main

import (
	"path/filepath"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/storage"
)

func TestRunCheck(t *testing.T) {
	fresh := fixtures.At(3 * time.Hour)
	store := &storage.Store{File: fixtures.Testdata("WowMcp_forever.lua"), History: &storage.History{Dir: t.TempDir(), Keep: 5}}
	if err := runCheck(store, fresh); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(&storage.Store{File: filepath.Join(t.TempDir(), "missing.lua")}, fresh); err == nil {
		t.Error("no error for a nonexistent file")
	}
}

func TestResolveGoalsFile(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "goals.json") // absolute on every OS ("/abs" is not absolute on Windows)
	if got := resolveGoalsFile(abs, "/x/history"); got != abs {
		t.Errorf("absolute path: %q", got)
	}
	if got := resolveGoalsFile("", filepath.Join("/srv", "wow", "history")); got != filepath.Join("/srv", "wow", "goals.json") {
		t.Errorf("default next to history: %q", got)
	}
	if got := resolveGoalsFile("", ""); filepath.Base(got) != "goals.json" {
		t.Errorf("without history, next to exe: %q", got)
	}
	// The game directory guard is not bypassed through the goals file.
	game := t.TempDir()
	if err := storage.RequireOutside("goals file (--goals-file)", filepath.Join(game, "goals.json"), game); err == nil {
		t.Error("a goals file inside the game directory should be rejected")
	}
}

// With only --file (no --wow-dir) the guard still knows the game directory, from the dump path.
func TestWriteGuardUsesTheGameDirectoryOfFile(t *testing.T) {
	game := filepath.Join(t.TempDir(), "World of Warcraft")
	dump := filepath.Join(game, "_forever_", "WTF", "Account", "ACC", "SavedVariables", storage.SavedVariablesFile)
	gameDirs := []string{"", storage.GameDirOf(dump)}
	if _, err := setupLogging("", filepath.Join(game, "history"), gameDirs); err == nil {
		t.Error("a history directory inside the game directory of --file was accepted")
	}
	if _, err := setupLogging(filepath.Join(game, "_forever_", "server.log"), filepath.Join(t.TempDir(), "history"), gameDirs); err == nil {
		t.Error("a log file inside the game directory of --file was accepted")
	}
	closeLog, err := setupLogging("", filepath.Join(t.TempDir(), "history"), gameDirs)
	if err != nil {
		t.Error(err)
	}
	closeLog()
}
