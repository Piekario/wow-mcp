// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package leftovers

import (
	"sort"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// readQuestItems collects quest items from bags and bank, by name, in name order.
func readQuestItems(s snapshot.Snapshot) []QuestItem {
	byName := map[string]*QuestItem{}
	for _, source := range []string{items.InBags, items.InBank} {
		for _, f := range items.FindItems(s, "", source) {
			if !f.Item.IsQuestItem() {
				continue
			}
			q := byName[f.Item.Name]
			if q == nil {
				q = &QuestItem{Name: f.Item.Name, Item: f.Item.Item}
				byName[f.Item.Name] = q
			}
			q.Count += f.Item.Count
			q.InBags = q.InBags || source == items.InBags
			q.InBank = q.InBank || source == items.InBank
		}
	}
	list := make([]QuestItem, 0, len(byName))
	for _, q := range byName {
		list = append(list, *q)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
