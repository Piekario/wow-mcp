// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package alerts

import (
	"wowmcp/internal/game/addon"
	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/mail"
	"wowmcp/internal/snapshot"
)

func read(s snapshot.Snapshot, fresh snapshot.Freshness) Assessment {
	a := Assessment{
		Unloaded:        s.IsUnloadedDump(),
		MailExpiring:    mail.Inspect(s, fresh).ExpiringSoonLabels(),
		AuctionsEnding:  auctions.EndingSoon(s, fresh, auctions.EndingSoonWithin),
		SlowestSections: addon.SlowestSections(s),
		ErrorCount:      len(snapshot.AsList(s["errors"])),
	}
	if report, ok := character.InspectDurability(s); ok {
		a.LowDurability = report.NearlyBrokenLabels()
	}
	space := items.Space(s)
	a.BagSlots, a.BagFree = space.Total, space.Free
	a.DumpMs, a.HasDumpMs = addon.DumpTimeMs(s)
	return a
}
