// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package doctor

import (
	"fmt"
	"time"

	"wowmcp/internal/game/addon"
	"wowmcp/internal/game/alerts"
	"wowmcp/internal/snapshot"
)

const englishOnlyHeuristics = "Some heuristics match English spell, zone and tooltip names (missing class buffs, flight points by zone, tooltip-based checks)"

// dump is what the checks look at in the character's dump.
type dump struct {
	HasDumpedAt      bool
	Age              time.Duration
	Stale            bool
	AddonVersion     string
	OutdatedAddon    bool
	Errors           []string
	DumpMs           float64
	HasDumpMs        bool
	SlowDump         bool
	SlowDumpMessage  string
	Disabled         []string
	DisabledNote     string
	Windows          []window
	Unloaded         bool
	KeptDumpOnLogout bool
	Locale           string
	EnglishOnlyNote  string
}

// window is a visit-based section: whether the dump has it and how to get it.
type window struct {
	Section  string
	Captured bool
	Open     any
	Tool     any
}

func read(s snapshot.Snapshot, fresh snapshot.Freshness) dump {
	d := dump{
		AddonVersion:    addon.Version(s),
		OutdatedAddon:   addon.VersionNote(s) != "",
		Disabled:        addon.DisabledSections(s),
		DisabledNote:    addon.DisabledNote(s),
		Unloaded:        s.IsUnloadedDump(),
		Locale:          s.Locale(),
		EnglishOnlyNote: s.EnglishOnlyNote(englishOnlyHeuristics),
	}
	if unix, ok := s.DumpedAt(); ok {
		d.HasDumpedAt, d.Age, d.Stale = true, fresh.Age(unix), fresh.IsStale(unix)
	}
	for _, e := range snapshot.AsList(s["errors"]) {
		d.Errors = append(d.Errors, fmt.Sprint(e))
	}
	d.DumpMs, d.HasDumpMs = addon.DumpTimeMs(s)
	if assessment := alerts.Assess(s, fresh); assessment.SlowDump() {
		d.SlowDump, d.SlowDumpMessage = true, assessment.SlowDumpMessage()
	}
	_, d.KeptDumpOnLogout = s["lastSkippedLogoutAt"]
	for _, raw := range addon.WindowCoverage(s, fresh) {
		w := snapshot.AsMap(raw)
		section, _ := w["section"].(string)
		captured, _ := w["captured"].(bool)
		d.Windows = append(d.Windows, window{Section: section, Captured: captured, Open: w["open"], Tool: w["tool"]})
	}
	return d
}
