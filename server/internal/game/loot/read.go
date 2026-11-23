// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package loot

import "wowmcp/internal/snapshot"

func readItems(period map[string]any) (items []Item) {
	for _, raw := range snapshot.AsList(period["items"]) {
		m := snapshot.AsMap(raw)
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		it := Item{Name: name, Quality: UnknownQuality, Count: snapshot.ItemCount(m)}
		it.ID, _ = snapshot.AsInt(m["id"])
		if q, ok := snapshot.AsInt(m["quality"]); ok {
			it.Quality = q
		}
		items = append(items, it)
	}
	return items
}

func itemsOf(raw any) []Item { return readItems(snapshot.AsMap(raw)) }

// Section returns the items from the loot section of one dump and the time from which the loot counts (since);
// false when the dump has no loot section (older addon or the section disabled). Only the current period:
// periods carried from earlier saves are merged by Gained.
func Section(s snapshot.Snapshot) (items []Item, since int64, ok bool) {
	sec := s.Map("loot")
	if sec == nil {
		return nil, 0, false
	}
	since, _ = snapshot.AsInt(sec["since"])
	return readItems(sec), since, true
}
