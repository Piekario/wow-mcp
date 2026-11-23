// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reagents

import (
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// readCarried is every item in the bags and the item in the ammo slot.
func readCarried(s snapshot.Snapshot) []itemkind.Item {
	var out []itemkind.Item
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			out = append(out, itemkind.Read(snapshot.AsMap(raw)))
		}
	}
	for _, raw := range s.Equipment() {
		if item := snapshot.AsMap(raw); item["slot"] == "ammo" {
			out = append(out, itemkind.Read(item))
		}
	}
	return out
}
