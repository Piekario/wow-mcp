// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package addon covers the WowMcp addon as seen from the dump: its version, the data
// collection time, disabled sections and the windows the player must open so the addon
// saves visit-based sections (trainer, bank, mail...).
package addon

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// ExpectedVersion is the addon version that collects every section this server uses.
const ExpectedVersion = "0.16.0"

// Version returns the version of the addon that made the dump ("" for a very old addon).
func Version(s snapshot.Snapshot) string {
	if v, _ := s["addonVersion"].(string); v != "" {
		return v
	}
	v, _ := s.Character()["addonVersion"].(string)
	return v
}

// VersionNote returns an update request when the dump comes from an older addon.
func VersionNote(s snapshot.Snapshot) string {
	if v := Version(s); v != "" && versionLess(v, ExpectedVersion) {
		return fmt.Sprintf("Dump from addon %s; the server supports %s (currencies, cooldowns, recipes, mail, played time, panel, note, deaths, dump timings, quest rewards, loot, stable, multiple vendors and trainers, XP by source, buyback list, equipment sets, PvP, friends and guild, zone and dungeon IDs, weapon speed, quest locations and guide steps). Copy the new addon into AddOns and run /reload.", v, ExpectedVersion)
	}
	return ""
}

// versionLess compares versions such as 0.2.0 < 0.3.0 numerically.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// DumpTimeMs is the time to collect the whole dump in milliseconds (field timings.totalMs); false for older addons.
func DumpTimeMs(s snapshot.Snapshot) (float64, bool) {
	return snapshot.AsFloat(s.Map("timings")["totalMs"])
}

// SlowestSections lists the slowest dump sections as "name 2.5 ms".
func SlowestSections(s snapshot.Snapshot) []string {
	var out []string
	for _, raw := range snapshot.AsList(s.Map("timings")["slowest"]) {
		e := snapshot.AsMap(raw)
		if ms, ok := snapshot.AsFloat(e["ms"]); ok {
			out = append(out, fmt.Sprintf("%v %g ms", e["name"], ms))
		}
	}
	return out
}

// DisabledSections lists the sections disabled in the addon with the /mcp off command.
func DisabledSections(s snapshot.Snapshot) []string {
	var out []string
	for _, raw := range snapshot.AsList(s["disabledSections"]) {
		if name, ok := raw.(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

// sectionTools says which tool reads a given section; sections outside the table are read by wow_section.
var sectionTools = map[string]string{
	"character": "wow_character", "stats": "wow_character", "auras": "wow_character", "professions": "wow_character",
	"talents": "wow_talents", "currencies": "wow_character", "deaths": "wow_character", "note": "wow_character",
	"quests": "wow_quests", "bags": "wow_bags", "equipment": "wow_equipment", "spells": "wow_spells", "pet": "wow_pet",
	"trainer": "wow_trainer", "reputation": "wow_reputation", "bank": "wow_bank", "vendor": "wow_vendor",
	"recipes": "wow_recipes", "mail": "wow_mail", "cooldowns": "wow_cooldowns", "auctions": "wow_auctions",
	"group": "wow_group", "inspect": "wow_group", "dungeons": "wow_dungeons", "played": "wow_progress",
	"vendors": "wow_vendor", "trainers": "wow_trainer", "stable": "wow_pet", "questOffer": "wow_quest_rewards",
	"questTurnIn": "wow_quest_rewards", "loot": "wow_journal", "xpGains": "wow_journal", "buyback": "wow_vendor",
	"equipmentSets": "wow_equipment", "pvp": "wow_character", "friends": "wow_social", "guild": "wow_social",
}

// ToolFor returns the tool that uses the section.
func ToolFor(section string) string {
	if tool, ok := sectionTools[section]; ok {
		return tool
	}
	return "wow_section"
}

// DisabledNote describes the effect of disabled sections: which tools lack data because of them. "" when nothing is disabled.
func DisabledNote(s snapshot.Snapshot) string {
	disabled := DisabledSections(s)
	if len(disabled) == 0 {
		return ""
	}
	parts := make([]string, len(disabled))
	for i, name := range disabled {
		parts[i] = fmt.Sprintf("%s (%s)", name, ToolFor(name))
	}
	return "Sections disabled in the addon: " + strings.Join(parts, ", ") +
		". Tools that use them have no fresh data; enable a section in game with the command \"/mcp on section_name\" and run /mcp reload."
}

// Describe collects the addon description for wow_status: version, update request, dump time and disabled sections.
func Describe(s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	if v := Version(s); v != "" {
		out["addonVersion"] = v
	}
	if note := VersionNote(s); note != "" {
		out["addonUpdate"] = note
	}
	if ms, ok := DumpTimeMs(s); ok {
		out["dumpTimeMs"] = ms
	}
	if disabled := DisabledSections(s); len(disabled) > 0 {
		out["disabledSections"] = disabled
		out["disabledNote"] = DisabledNote(s)
	}
	return out
}
