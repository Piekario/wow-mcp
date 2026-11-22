// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package doctor is the diagnostics context: it looks at the dump and at facts about the server's surroundings
// (file, accounts, history, goals) and answers "why does something look wrong, and what do I do about it".
// It does no I/O: the adapter gathers the facts and passes them in.
package doctor

import (
	"fmt"
	"strings"
	"time"

	"wowmcp/internal/game/addon"
	"wowmcp/internal/game/alerts"
	"wowmcp/internal/snapshot"
)

// Status of one check.
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

// Check is one diagnostic line.
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
	checks := []Check{
		dumpFile(f, fresh),
		accountCheck(f),
		dumpAge(s, fresh),
		addonVersion(s),
		sectionErrors(s),
		dumpSpeed(s, fresh),
		disabledSections(s),
		windowSections(s, fresh),
		unloadedDump(s, f),
		clientLanguage(s),
		historyCheck(f),
		goalsCheck(f),
	}
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

func dumpFile(f Facts, fresh snapshot.Freshness) Check {
	c := Check{Check: "dump file", Status: OK}
	c.Detail = "Reading " + f.File
	if !f.FileModified.IsZero() {
		age := fresh.Now().Sub(f.FileModified)
		c.Detail += fmt.Sprintf(" (modified %s, %s ago)", f.FileModified.Format("2006-01-02 15:04:05"), snapshot.FormatAge(max(age, 0)))
	}
	return c
}

func accountCheck(f Facts) Check {
	c := Check{Check: "account", Status: OK}
	switch {
	case f.Account == "":
		c.Detail = "The account folder is not part of the path (--file was used), so there is nothing to choose between."
	case len(f.OtherAccounts) == 0:
		c.Detail = fmt.Sprintf("Account %q is the only one with a dump.", f.Account)
	case f.AccountPinned:
		c.Detail = fmt.Sprintf("Account %q is pinned (--account / WOW_ACCOUNT). Other accounts with a dump: %s.", f.Account, strings.Join(f.OtherAccounts, ", "))
	default:
		c.Status = Warn
		c.Detail = fmt.Sprintf("Several accounts have a dump; the most recently modified one is used: %q. Others: %s.", f.Account, strings.Join(f.OtherAccounts, ", "))
		c.Fix = "If characters are missing, pin the right account with --account <folder name> (or the WOW_ACCOUNT variable) and restart the server."
	}
	return c
}

func dumpAge(s snapshot.Snapshot, fresh snapshot.Freshness) Check {
	c := Check{Check: "dump age", Status: OK}
	unix, ok := s.DumpedAt()
	if !ok {
		c.Status, c.Detail = Warn, "The dump has no time stamp (a very old addon)."
		c.Fix = "Update the addon (copy it into AddOns) and type /reload in game."
		return c
	}
	age := fresh.Age(unix)
	c.Detail = fmt.Sprintf("The snapshot is %s old.", snapshot.FormatAge(age))
	if fresh.IsStale(unix) {
		c.Status = Warn
		c.Detail += " That is older than the stale threshold, so advice may be out of date."
		c.Fix = "Type /mcp reload in game (or click Save on the WowMcp panel); the game writes the file only on /reload, logout and exit."
	}
	return c
}

func addonVersion(s snapshot.Snapshot) Check {
	c := Check{Check: "addon version", Status: OK}
	v := addon.Version(s)
	if v == "" {
		c.Status, c.Detail = Warn, "The dump does not say which addon version wrote it (a very old addon)."
		c.Fix = "Copy the current addon into AddOns and type /reload in game."
		return c
	}
	c.Detail = fmt.Sprintf("Addon %s, the server expects %s.", v, addon.ExpectedVersion)
	if note := addon.VersionNote(s); note != "" {
		c.Status = Warn
		c.Fix = "Update the addon: copy the new WowMcp folder into Interface/AddOns (install.ps1 does it) and type /reload in game."
	}
	return c
}

func sectionErrors(s snapshot.Snapshot) Check {
	c := Check{Check: "section errors", Status: OK, Detail: "The addon reported no errors while collecting sections."}
	errs := snapshot.AsList(s["errors"])
	if len(errs) == 0 {
		return c
	}
	shown := make([]string, 0, 3)
	for i, e := range errs {
		if i == 3 {
			break
		}
		shown = append(shown, fmt.Sprint(e))
	}
	c.Status = Warn
	c.Detail = fmt.Sprintf("The addon reported %d error(s) while collecting sections; the affected sections may be missing or partial. First: %s", len(errs), strings.Join(shown, " | "))
	c.Fix = "Type /mcp reload again; if the errors stay, report them (the full list is in wow_status as addonErrors) together with the addon and client versions."
	return c
}

func dumpSpeed(s snapshot.Snapshot, fresh snapshot.Freshness) Check {
	c := Check{Check: "dump speed", Status: OK}
	ms, ok := addon.DumpTimeMs(s)
	if !ok {
		c.Detail = "The dump does not report its collection time (addon older than 0.5.0)."
		return c
	}
	c.Detail = fmt.Sprintf("Collecting the dump took %g ms.", ms)
	for _, a := range alerts.For(s, fresh) {
		if a.Kind == "slowDump" {
			c.Status, c.Detail = Warn, a.Message+"."
			c.Fix = "Turn off the slowest section you do not need with /mcp off <section> (see disabled sections below), then /mcp reload."
		}
	}
	return c
}

func disabledSections(s snapshot.Snapshot) Check {
	c := Check{Check: "disabled sections", Status: OK, Detail: "No section is turned off in the addon."}
	if len(addon.DisabledSections(s)) == 0 {
		return c
	}
	c.Status, c.Detail = Warn, addon.DisabledNote(s)
	c.Fix = "Turn a section back on with /mcp on <section> and type /mcp reload."
	return c
}

// optionalWindows are visit-based sections that are expected to be missing for many players.
var optionalWindows = map[string]bool{"guild": true, "inspect": true}

func windowSections(s snapshot.Snapshot, fresh snapshot.Freshness) Check {
	c := Check{Check: "window sections", Status: OK, Detail: "Every window-based section has been captured at least once."}
	var missing []string
	for _, raw := range addon.WindowCoverage(s, fresh) {
		w := snapshot.AsMap(raw)
		section, _ := w["section"].(string)
		if captured, _ := w["captured"].(bool); captured || optionalWindows[section] {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (open: %v; read by %v)", section, w["open"], w["tool"]))
	}
	if len(missing) == 0 {
		return c
	}
	c.Status = Warn
	c.Detail = fmt.Sprintf("%d section(s) were never captured because their window was not opened with the addon enabled: %s", len(missing), strings.Join(missing, "; "))
	c.Fix = "Open the windows you want answers about (vendor, trainer, bank, mailbox, auction house, professions...) once, then type /mcp reload."
	return c
}

func unloadedDump(s snapshot.Snapshot, f Facts) Check {
	c := Check{Check: "unloaded dump", Status: OK}
	switch {
	case s.IsUnloadedDump():
		c.Status = Fail
		c.Detail = "This dump was written while exiting the game, when the client no longer had character data (0 gold, empty bags and spellbook); it does not describe the character."
		c.Fix = "Log in with the character and type /mcp reload. Addon 0.7.1 and newer no longer saves such dumps."
		return c
	case f.HistorySkippedUnloaded > 0:
		c.Detail = fmt.Sprintf("The history skips %d dump(s) written on game exit (empty client); the files stay on disk but do not count in progress or changes.", f.HistorySkippedUnloaded)
	default:
		c.Detail = "The dump holds a loaded character."
	}
	if _, ok := s["lastSkippedLogoutAt"]; ok {
		c.Detail += " The addon kept the previous dump when the game was last closed."
	}
	return c
}

func clientLanguage(s snapshot.Snapshot) Check {
	c := Check{Check: "client language", Status: OK}
	if s.Locale() == "" {
		c.Detail = "The dump does not report the client locale (addon older than 0.9.0); an English client is assumed."
		return c
	}
	c.Detail = "Client locale " + s.Locale() + "."
	if note := s.EnglishOnlyNote("Some heuristics match English spell, zone and tooltip names (missing class buffs, flight points by zone, tooltip-based checks)"); note != "" {
		c.Status, c.Detail = Warn, "Client locale "+s.Locale()+". "+note
		c.Fix = "Nothing to repair: item, stat and profession logic uses IDs and works. Treat results of the name-based heuristics as incomplete; names stay as the client shows them."
	}
	return c
}

func historyCheck(f Facts) Check {
	c := Check{Check: "history", Status: OK}
	switch {
	case f.HistoryDir == "":
		c.Status = Warn
		c.Detail = "History is disabled, so wow_changes, wow_progress, wow_gold_history and the timeline have nothing to compare."
		c.Fix = "Start the server with a --history-dir (the default is the history folder next to the executable)."
	case f.HistoryError != "":
		c.Status = Fail
		c.Detail = fmt.Sprintf("The history directory %s cannot be written: %s", f.HistoryDir, f.HistoryError)
		c.Fix = "Make the directory writable or point --history-dir to another folder outside the game directory."
	default:
		c.Detail = f.HistoryDir + " is writable."
		if f.Character != "" {
			c.Detail = fmt.Sprintf("%s is writable; %d usable snapshot(s) of %s.", f.HistoryDir, f.HistoryEntries, f.Character)
			if f.HistoryEntries < 2 {
				c.Detail += " Progress tools need at least two snapshots: they appear after more /mcp reload cycles."
			}
		}
	}
	return c
}

func goalsCheck(f Facts) Check {
	c := Check{Check: "goals file", Status: OK}
	if f.GoalsFile == "" {
		c.Detail = "Goals are disabled (no --goals-file); wow_goals and wow_goal_set return an error."
		return c
	}
	c.Detail = "Goals are stored in " + f.GoalsFile + "."
	return c
}
