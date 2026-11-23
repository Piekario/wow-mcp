// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package alerts

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// For returns the warnings for a character dump; an empty list when nothing applies.
func For(s snapshot.Snapshot, fresh snapshot.Freshness) []Alert {
	return Assess(s, fresh).Alerts()
}

// Alerts are the warnings in display order.
func (a Assessment) Alerts() []Alert {
	out := []Alert{}
	add := func(applies bool, kind string, message func() string) {
		if applies {
			out = append(out, Alert{kind, message()})
		}
	}
	add(a.Unloaded, KindUnloadedDump, func() string {
		return "This dump comes from exiting the game, when the client no longer had character data (0 gold, empty bags and spellbook). " +
			"This data does not describe the character: log in and type /mcp reload. Addon 0.7.1 no longer saves such dumps"
	})
	add(len(a.MailExpiring) > 0, KindMail, func() string {
		return "Mail expires within 3 days: " + strings.Join(a.MailExpiring, "; ")
	})
	add(len(a.AuctionsEnding) > 0, KindAuctions, func() string {
		return "Your auctions end within 2 h: " + strings.Join(a.AuctionsEnding, "; ")
	})
	add(len(a.LowDurability) > 0, KindDurability, func() string {
		return "Low durability, worth repairing: " + strings.Join(a.LowDurability, ", ")
	})
	add(a.BagsAlmostFull(), KindBags, func() string {
		return fmt.Sprintf("Bags almost full: %d of %d slots free", a.BagFree, a.BagSlots)
	})
	add(a.SlowDump(), KindSlowDump, a.SlowDumpMessage)
	add(a.ErrorCount > 0, KindAddonErrors, func() string {
		return fmt.Sprintf("The addon reported errors while collecting sections (count: %d, field addonErrors)", a.ErrorCount)
	})
	return out
}

// SlowDumpMessage explains a slow dump and names the slowest sections.
func (a Assessment) SlowDumpMessage() string {
	msg := fmt.Sprintf("The addon dump took %g ms (over %d ms); the game may stutter briefly on /mcp reload", a.DumpMs, slowDumpMs)
	if len(a.SlowestSections) > 0 {
		msg += ". Slowest sections: " + strings.Join(a.SlowestSections, ", ")
	}
	return msg
}

// Messages flattens the warnings into a list of short texts (for prompts).
func Messages(list []Alert) []string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.Message
	}
	return out
}
