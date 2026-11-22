// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// diffXPGains describes the XP gained after the older dump by source (kills, quests, the rest): the xpGains section carries
// what came in since the previous write to disk and the closed periods of saves the server never read, like loot.
// nil when the dump has no section or nothing was gained.
func diffXPGains(before, after snapshot.Snapshot) map[string]any {
	g, m := xpgains.Gained(before, []snapshot.Snapshot{after})
	if !m.Found {
		return nil
	}
	out := xpgains.Describe(g)
	if out == nil {
		return nil
	}
	coverage(out, before, m, "XP from earlier saves between these dumps is not included: the xpGains section of the newer dump counts from that moment")
	return out
}
