// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// nearlyBroken: below this percentage an item goes on the repair list.
const nearlyBroken = 25

// Durability collects equipment durability: the average, the weakest item and the repair
// cost quoted by the last vendor. nil when nothing has durability.
func Durability(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	var cur, max int64
	lowest := 101.0
	var lowestName string
	var low []string
	for _, raw := range s.Equipment() {
		item := snapshot.AsMap(raw)
		c, ok1 := snapshot.AsInt(item["durability"])
		m, ok2 := snapshot.AsInt(item["durabilityMax"])
		if !ok1 || !ok2 || m <= 0 {
			continue
		}
		cur += c
		max += m
		pct := float64(c*1000/m) / 10
		if pct < lowest {
			lowest, lowestName = pct, fmt.Sprint(item["name"])
		}
		if pct < nearlyBroken {
			low = append(low, fmt.Sprintf("%v %g%%", item["name"], pct))
		}
	}
	if max == 0 {
		return nil
	}
	out := map[string]any{
		"overallPercent": float64(cur*1000/max) / 10,
		"lowest":         fmt.Sprintf("%s %g%%", lowestName, lowest),
	}
	if len(low) > 0 {
		out["nearlyBroken"] = low
		out["advice"] = "Below 25% it is worth repairing before the next outing; at 0% the item stops working."
	}
	if v := s.Map("vendor"); v != nil {
		if cost, ok := snapshot.AsInt(v["repairCostCopper"]); ok {
			r := map[string]any{"cost": snapshot.Money(cost).String(), "at": v["npc"]}
			snapshot.Merge(r, fresh.CapturedMeta(v))
			out["lastRepairQuote"] = r
		}
	}
	return out
}
