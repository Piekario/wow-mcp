// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// buybackAssumption: what the server does not know about the buyback list.
const buybackAssumption = "The buyback list is an assumption, not a certainty: the addon saves it from the vendor window, and the client keeps it only until logout, " +
	"so after a relog the items on the list may no longer be buyable (the server cannot see this; see capturedAge). " +
	"priceCopper is the price for the whole batch (count units), as in the vendor offer."

// Buyback describes the buyback list from the last vendor window (the buyback section): what can still be bought back, for how much,
// whether the character can afford it and how long ago the list was saved. nil when the dump has no section. query narrows by item name.
func Buyback(s snapshot.Snapshot, query string, fresh snapshot.Freshness) map[string]any {
	sec := s.Map("buyback")
	if sec == nil {
		return nil
	}
	_, money := s.LevelAndMoney()
	needle := strings.ToLower(query)
	list := []any{}
	var total int64
	for _, raw := range snapshot.AsList(sec["items"]) {
		item := snapshot.AsMap(raw)
		name, _ := item["name"].(string)
		if needle != "" && !strings.Contains(strings.ToLower(name), needle) {
			continue
		}
		e := map[string]any{"name": name}
		for _, k := range []string{"index", "id", "count"} {
			if v, ok := item[k]; ok {
				e[k] = v
			}
		}
		if price, ok := snapshot.AsInt(item["priceCopper"]); ok {
			e["priceCopper"] = price
			e["price"] = snapshot.Money(price).String()
			e["canAfford"] = price <= money
			total += price
		}
		list = append(list, e)
	}
	out := map[string]any{"npc": sec["npc"], "items": list, "count": len(list), "assumption": buybackAssumption}
	if len(list) > 0 {
		out["totalPrice"] = snapshot.Money(total).String()
	}
	snapshot.Merge(out, fresh.CapturedMeta(sec))
	if at, ok := snapshot.AsInt(sec["capturedAt"]); ok && fresh.Now != nil {
		out["capturedAge"] = snapshot.FormatAge(fresh.Age(at))
	}
	return out
}
