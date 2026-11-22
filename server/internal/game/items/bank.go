// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "wowmcp/internal/snapshot"

// BankQuery holds the bank content filters (wow_bank).
type BankQuery struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of an item name"`
	Type       string `json:"type,omitempty" jsonschema:"item type or subtype, e.g. Weapon, Armor, Trade Goods, Cloth"`
	MinQuality int64  `json:"minQuality,omitempty" jsonschema:"minimum quality: 0 Poor, 1 Common, 2 Uncommon, 3 Rare, 4 Epic"`
	Sellable   bool   `json:"sellable,omitempty" jsonschema:"true = only items a vendor will buy (sell price > 0)"`
}

// Bank describes the bank from the last visit: items with their vendor value and free slots.
func Bank(s snapshot.Snapshot, q BankQuery, fresh snapshot.Freshness) map[string]any {
	bank := s.Map("bank")
	if bank == nil {
		return map[string]any{"note": "No saved bank. Open the bank in game, then type /mcp reload."}
	}
	f := filter{query: q.Query, kind: q.Type, minQuality: q.MinQuality}
	items := []any{}
	var totalSlots, freeSlots, totalValue int64
	for _, rawC := range snapshot.AsList(bank["containers"]) {
		c := snapshot.AsMap(rawC)
		size, _ := snapshot.AsInt(c["size"])
		free, _ := snapshot.AsInt(c["free"])
		totalSlots += size
		freeSlots += free
		for _, rawI := range snapshot.AsList(c["items"]) {
			item := snapshot.AsMap(rawI)
			if !f.match(item) {
				continue
			}
			sell, _ := snapshot.AsInt(item["sellPrice"])
			if q.Sellable && sell <= 0 {
				continue
			}
			count := snapshot.ItemCount(item)
			entry := copyItem(item)
			entry["container"] = c["name"]
			if name := qualityName(item); name != "" {
				entry["qualityName"] = name
			}
			if sell > 0 {
				entry["vendorValue"] = snapshot.Money(sell * count).String()
				totalValue += sell * count
			}
			items = append(items, entry)
		}
	}
	out := map[string]any{
		"items":            items,
		"count":            len(items),
		"totalSlots":       totalSlots,
		"freeSlots":        freeSlots,
		"totalVendorValue": snapshot.Money(totalValue).String(),
	}
	snapshot.Merge(out, fresh.CapturedMeta(bank))
	return out
}
