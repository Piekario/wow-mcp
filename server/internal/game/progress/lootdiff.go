// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/periods"
	"wowmcp/internal/snapshot"
)

// maxLootListed is how many notable loot entries wow_changes shows.
const maxLootListed = 10

// diffLoot describes the player's own loot gained after the older dump: the loot section of the newer dump carries what
// dropped since the previous write to disk, and since addon 0.12.0 also the closed periods of saves the server never read
// (merged when they started at or after the older dump). nil when the dump has no section or no loot.
func diffLoot(before, after snapshot.Snapshot) map[string]any {
	items, m := loot.Gained(before, []snapshot.Snapshot{after})
	if !m.Found || len(items) == 0 {
		return nil
	}
	out := loot.Describe(items, maxLootListed)
	coverage(out, before, m, "loot from earlier saves between these dumps is not included: the loot section of the newer dump counts from that moment")
	return out
}

// coverage adds since (the start of the earliest merged period), the number of merged carried periods and,
// when the merged periods start after the older dump, the partialNote.
func coverage(out map[string]any, before snapshot.Snapshot, m periods.Merge, partialNote string) {
	since := m.Earliest()
	if since <= 0 {
		return
	}
	out["since"] = snapshot.FormatTime(since)
	if from, ok := before.DumpedAt(); ok && since > from {
		out["partialNote"] = partialNote
	}
	m.Describe(out)
}
