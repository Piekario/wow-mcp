// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// BagSpace returns the total of bag slots.
func BagSpace(s snapshot.Snapshot) map[string]any {
	var total, free int64
	for _, raw := range s.Bags() {
		b := snapshot.AsMap(raw)
		n, _ := snapshot.AsInt(b["size"])
		f, _ := snapshot.AsInt(b["free"])
		total += n
		free += f
	}
	return map[string]any{"totalSlots": total, "freeSlots": free}
}

// SummarizeBags adds the slot totals, junk and ammo to the wow_bags response.
func SummarizeBags(s snapshot.Snapshot, out map[string]any) {
	snapshot.Merge(out, BagSpace(s))
	snapshot.Merge(out, bagExtras(s))
}

// bagExtras adds the junk value (Poor quality) and ammo to the bags.
func bagExtras(s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	var junk int64
	var junkItems []string
	for _, rawBag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(rawBag)["items"]) {
			item := snapshot.AsMap(raw)
			if q, ok := snapshot.AsInt(item["quality"]); !ok || q != 0 {
				continue
			}
			sell, _ := snapshot.AsInt(item["sellPrice"])
			junk += sell * snapshot.ItemCount(item)
			junkItems = append(junkItems, fmt.Sprint(item["name"]))
		}
	}
	if len(junkItems) > 0 {
		out["junk"] = map[string]any{"items": junkItems, "vendorValue": snapshot.Money(junk).String()}
	}
	for _, raw := range s.Equipment() {
		if item := snapshot.AsMap(raw); item["slot"] == "ammo" {
			out["ammo"] = map[string]any{"name": item["name"], "count": item["count"]}
		}
	}
	return out
}
