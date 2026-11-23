// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package alerts gathers in one place everything that needs the player's attention: expiring
// mail, auctions about to end, damaged gear, full bags and addon problems.
// Shown in wow_status; an empty list means nothing is on fire.
package alerts

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/addon"
	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/mail"
	"wowmcp/internal/snapshot"
)

// Warning thresholds.
const (
	lowFreeSlots = 2   // this many free bag slots or fewer
	slowDumpMs   = 200 // dump collected for longer than this many milliseconds
)

// Alert is a single warning: its kind (for filtering) and the text for the player.
type Alert struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// check is a single rule; it returns a warning, or false when nothing applies to the character.
type check func(s snapshot.Snapshot, fresh snapshot.Freshness) (Alert, bool)

// checks are the rules in display order.
var checks = []check{unloadedDump, mailExpiring, auctionsEnding, lowDurability, bagsAlmostFull, slowDump, addonErrors}

// For returns the warnings for a character dump; an empty list when nothing applies.
func For(s snapshot.Snapshot, fresh snapshot.Freshness) []Alert {
	out := []Alert{}
	for _, c := range checks {
		if a, ok := c(s, fresh); ok {
			out = append(out, a)
		}
	}
	return out
}

// unloadedDump: a dump taken on exit from the game, when the client no longer had character data (addon older than 0.7.1).
func unloadedDump(s snapshot.Snapshot, _ snapshot.Freshness) (Alert, bool) {
	if !s.IsUnloadedDump() {
		return Alert{}, false
	}
	return Alert{"unloadedDump", "This dump comes from exiting the game, when the client no longer had character data (0 gold, empty bags and spellbook). " +
		"This data does not describe the character: log in and type /mcp reload. Addon 0.7.1 no longer saves such dumps"}, true
}

func mailExpiring(s snapshot.Snapshot, fresh snapshot.Freshness) (Alert, bool) {
	expiring := mail.Inspect(s, fresh).ExpiringSoonLabels()
	if len(expiring) == 0 {
		return Alert{}, false
	}
	return Alert{"mail", "Mail expires within 3 days: " + strings.Join(expiring, "; ")}, true
}

func auctionsEnding(s snapshot.Snapshot, fresh snapshot.Freshness) (Alert, bool) {
	ending := auctions.EndingSoon(s, fresh, auctions.EndingSoonWithin)
	if len(ending) == 0 {
		return Alert{}, false
	}
	return Alert{"auctions", "Your auctions end within 2 h: " + strings.Join(ending, "; ")}, true
}

func lowDurability(s snapshot.Snapshot, fresh snapshot.Freshness) (Alert, bool) {
	low, _ := character.Durability(s, fresh)["nearlyBroken"].([]string)
	if len(low) == 0 {
		return Alert{}, false
	}
	return Alert{"durability", "Low durability, worth repairing: " + strings.Join(low, ", ")}, true
}

func bagsAlmostFull(s snapshot.Snapshot, _ snapshot.Freshness) (Alert, bool) {
	space := items.BagSpace(s)
	total, free := space["totalSlots"].(int64), space["freeSlots"].(int64)
	if total == 0 || free > lowFreeSlots {
		return Alert{}, false
	}
	return Alert{"bags", fmt.Sprintf("Bags almost full: %d of %d slots free", free, total)}, true
}

func slowDump(s snapshot.Snapshot, _ snapshot.Freshness) (Alert, bool) {
	ms, ok := addon.DumpTimeMs(s)
	if !ok || ms <= slowDumpMs {
		return Alert{}, false
	}
	msg := fmt.Sprintf("The addon dump took %g ms (over %d ms); the game may stutter briefly on /mcp reload", ms, slowDumpMs)
	if slow := addon.SlowestSections(s); len(slow) > 0 {
		msg += ". Slowest sections: " + strings.Join(slow, ", ")
	}
	return Alert{"slowDump", msg}, true
}

func addonErrors(s snapshot.Snapshot, _ snapshot.Freshness) (Alert, bool) {
	errs := snapshot.AsList(s["errors"])
	if len(errs) == 0 {
		return Alert{}, false
	}
	return Alert{"addonErrors", fmt.Sprintf("The addon reported errors while collecting sections (count: %d, field addonErrors)", len(errs))}, true
}

// Messages flattens the warnings into a list of short texts (for prompts).
func Messages(list []Alert) []string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.Message
	}
	return out
}
