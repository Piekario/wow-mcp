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
const ExpectedVersion = "0.20.0"

// SectionTiming is how long the addon took over one section. Name is kept as the dump has it.
type SectionTiming struct {
	Name any
	Ms   float64
}

// Info is what the dump says about the addon that made it.
type Info struct {
	Version  string
	TotalMs  float64
	HasTotal bool
	Slowest  []SectionTiming
	Disabled []string
}

// OutOfDate says whether the dump comes from an older addon than the server supports.
func (i Info) OutOfDate() bool { return i.Version != "" && versionLess(i.Version, ExpectedVersion) }

// WindowStatus is a visit-based section: whether the dump has it and when it was saved.
type WindowStatus struct {
	window
	Present  bool
	Captured snapshot.Captured
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
	"achievements": "wow_achievements", "statistics": "wow_statistics", "titles": "wow_collections", "collections": "wow_collections",
}

// ToolFor returns the tool that uses the section.
func ToolFor(section string) string {
	if tool, ok := sectionTools[section]; ok {
		return tool
	}
	return "wow_section"
}
