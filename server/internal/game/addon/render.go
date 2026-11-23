// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package addon

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// VersionNote returns an update request when the dump comes from an older addon.
func VersionNote(s snapshot.Snapshot) string { return read(s).versionNote() }

func (i Info) versionNote() string {
	if !i.OutOfDate() {
		return ""
	}
	return fmt.Sprintf("Dump from addon %s; the server supports %s (currencies, cooldowns, recipes, mail, played time, panel, note, deaths, dump timings, quest rewards, loot, stable, multiple vendors and trainers, XP by source, buyback list, equipment sets, PvP, friends and guild, zone and dungeon IDs, weapon speed, quest locations and guide steps). Copy the new addon into AddOns and run /reload.", i.Version, ExpectedVersion)
}

// SlowestSections lists the slowest dump sections as "name 2.5 ms".
func SlowestSections(s snapshot.Snapshot) []string {
	var out []string
	for _, t := range read(s).Slowest {
		out = append(out, fmt.Sprintf("%v %g ms", t.Name, t.Ms))
	}
	return out
}

// DisabledNote describes the effect of disabled sections: which tools lack data because of them. "" when nothing is disabled.
func DisabledNote(s snapshot.Snapshot) string { return read(s).disabledNote() }

func (i Info) disabledNote() string {
	if len(i.Disabled) == 0 {
		return ""
	}
	parts := make([]string, len(i.Disabled))
	for n, name := range i.Disabled {
		parts[n] = fmt.Sprintf("%s (%s)", name, ToolFor(name))
	}
	return "Sections disabled in the addon: " + strings.Join(parts, ", ") +
		". Tools that use them have no fresh data; enable a section in game with the command \"/mcp on section_name\" and run /mcp reload."
}

// Describe collects the addon description for wow_status: version, update request, dump time and disabled sections.
func Describe(s snapshot.Snapshot) map[string]any {
	info := read(s)
	out := map[string]any{}
	if info.Version != "" {
		out["addonVersion"] = info.Version
	}
	if note := info.versionNote(); note != "" {
		out["addonUpdate"] = note
	}
	if info.HasTotal {
		out["dumpTimeMs"] = info.TotalMs
	}
	if len(info.Disabled) > 0 {
		out["disabledSections"] = info.Disabled
		out["disabledNote"] = info.disabledNote()
	}
	return out
}

// RefreshHint says how to refresh data in game; the window list comes from the windows table.
func RefreshHint() string {
	names := make([]string, len(windows))
	opened := make([]string, len(windows))
	for i, w := range windows {
		names[i], opened[i] = w.section, w.opened
	}
	return "In game type /mcp reload or click Save on the WowMcp panel. Sections " + strings.Join(names, ", ") +
		" are saved when the character opens: " + strings.Join(opened, "; ") + "; then /mcp reload."
}

// WindowCoverage says which visit-based sections are already in the dump, which are missing and what to open in game.
func WindowCoverage(s snapshot.Snapshot, fresh snapshot.Freshness) []any {
	statuses := readWindows(s)
	out := make([]any, len(statuses))
	for i, w := range statuses {
		e := map[string]any{"section": w.section, "open": w.opened, "tool": w.tool, "captured": w.Present}
		if w.Present {
			snapshot.Merge(e, fresh.CapturedMetaOf(w.Captured))
		}
		out[i] = e
	}
	return out
}
