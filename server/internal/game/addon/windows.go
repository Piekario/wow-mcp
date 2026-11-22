// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package addon

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// window is a section saved only after a window is opened in game (the state "from the last visit").
type window struct {
	section string
	opened  string // what the player must open in game
	tool    string
}

// windows are the visit-based sections, in the order worth showing them.
var windows = []window{
	{"trainer", "class or profession trainer window (the addon remembers up to 10 latest in the trainers field)", "wow_trainer"},
	{"vendor", "vendor window (preferably one that repairs; up to 10 latest in the vendors field)", "wow_vendor"},
	{"buyback", "vendor window after selling something (buyback list; the client loses it on logout)", "wow_vendor"},
	{"bank", "bank window", "wow_bank"},
	{"flightPaths", "flight map at a flight master", "wow_section flightPaths"},
	{"recipes", "each profession window (and Beast Training for a Hunter)", "wow_recipes"},
	{"mail", "mailbox", "wow_mail"},
	{"auctions", "auction house: search for items and open the Auctions tab", "wow_auctions"},
	{"stable", "stable window (Stable Master)", "wow_pet"},
	{"questOffer", "quest window at an NPC, before you accept it", "wow_quest_rewards"},
	{"questTurnIn", "quest turn-in window at an NPC (with the reward choice)", "wow_quest_rewards"},
	{"friends", "social window with the friends list (the addon does not request the list from the server itself)", "wow_social"},
	{"guild", "guild window (only when the character is in a guild; the addon does not request the list from the server itself)", "wow_social"},
	{"inspect", "Inspect window on another player", "wow_group"},
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
	out := make([]any, len(windows))
	for i, w := range windows {
		e := map[string]any{"section": w.section, "open": w.opened, "tool": w.tool}
		section, present := s[w.section]
		e["captured"] = present
		if present {
			snapshot.Merge(e, fresh.CapturedMeta(snapshot.AsMap(section)))
		}
		out[i] = e
	}
	return out
}
