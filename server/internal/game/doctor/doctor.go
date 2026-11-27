// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package doctor is the diagnostics context: it looks at the dump and at facts about the server's surroundings
// (file, accounts, history, goals) and answers "why does something look wrong, and what do I do about it".
// It does no I/O: the adapter gathers the facts and passes them in.
package doctor

import (
	"fmt"
	"time"

	"wowmcp/internal/game/addon"
	"wowmcp/internal/snapshot"
)

const (
	OK   = "ok"
	Warn = "warn"
	Fail = "fail"
)

// Facts are the things about the server's surroundings that the domain cannot read itself.
type Facts struct {
	ServerVersion string

	File         string    // the dump file that was read
	FileModified time.Time // zero = unknown

	Account       string   // WTF account folder the dump came from ("" = unknown)
	OtherAccounts []string // other account folders that have a dump
	AccountPinned bool     // chosen with --account / WOW_ACCOUNT

	Character string // the character the diagnosis is about

	HistoryDir             string // "" = history disabled
	HistoryError           string // "" = the directory is writable
	HistoryEntries         int    // snapshots of the character usable by wow_changes and friends
	HistorySkippedUnloaded int    // snapshots of the character skipped as dumps from an exited game

	GoalsFile string // "" = goals disabled
}

type Check struct {
	Check  string `json:"check"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

// Report is the answer of wow_doctor.
type Report struct {
	Status   string  `json:"status"` // the worst status among the checks
	Summary  string  `json:"summary"`
	NextStep string  `json:"nextStep"`
	Checks   []Check `json:"checks"`
}

// Diagnose checks a readable dump.
func Diagnose(s snapshot.Snapshot, f Facts, fresh snapshot.Freshness) Report {
	d := read(s, fresh)
	checks := []Check{
		dumpFile(f, fresh),
		accountCheck(f),
		dumpAge(d),
		addonVersion(d),
		sectionErrors(d),
		dumpSpeed(d),
		disabledSections(d),
		windowSections(d),
		unloadedDump(d, f),
		clientLanguage(d),
	}
	if d.HasClassGroups {
		checks = append(checks, classBuffIDs(d))
	}
	checks = append(checks, historyCheck(f), goalsCheck(f))
	return build(checks, f)
}

// Unreadable is the report when the dump file could not be found or read; loadErr is the reason.
func Unreadable(loadErr string, f Facts) Report {
	checks := []Check{
		{Check: "dump file", Status: Fail, Detail: "The dump could not be read: " + loadErr,
			Fix: "Check --wow-dir (or WOW_DIR) and --client / --account if you use them, make sure the WowMcp addon is enabled, " +
				"log in with a character and type /mcp reload in game; then call wow_doctor again."},
		historyCheck(f),
		goalsCheck(f),
	}
	return build(checks, f)
}

func build(checks []Check, f Facts) Report {
	counts := map[string]int{}
	worst := OK
	next := ""
	for _, c := range checks {
		counts[c.Status]++
		switch {
		case c.Status == Fail && worst != Fail:
			worst, next = Fail, c.Fix
		case c.Status == Warn && worst == OK:
			worst, next = Warn, c.Fix
		}
	}
	if next == "" {
		next = "Nothing to fix. If an answer still looks wrong, ask for wow_status (alerts) and the section in question (wow_section)."
	}
	summary := fmt.Sprintf("%d checks: %d ok, %d warnings, %d failures (server %s, expected addon %s)",
		len(checks), counts[OK], counts[Warn], counts[Fail], orUnknown(f.ServerVersion), addon.ExpectedVersion)
	return Report{Status: worst, Summary: summary, NextStep: next, Checks: checks}
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
