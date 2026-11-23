// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"maps"
	"slices"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// detailKeys are the item details the addon saves with a price (0.13.0, from the client cache only).
var detailKeys = []string{"type", "subType", "classID", "subclassID", "equipLoc", "itemLevel", "requiredLevel", "quality", "sellPrice", "bindType", "dps", "statTokens"}

var passedKeys = []string{"id", "quantity", "listings", "source"}

func readSection(s snapshot.Snapshot) Section {
	m := s.Map("auctions")
	sec := Section{Present: m != nil}
	prices := snapshot.AsMap(m["prices"])
	for _, key := range slices.Sorted(maps.Keys(prices)) {
		sec.Prices = append(sec.Prices, readPrice(key, prices[key]))
	}
	owned := snapshot.AsList(m["owned"])
	sec.HasOwned = owned != nil
	for _, raw := range owned {
		sec.Owned = append(sec.Owned, readOwnAuction(snapshot.AsMap(raw)))
	}
	sec.OwnedAt, sec.HasOwnedAt = snapshot.AsInt(m["ownedAt"])
	sec.LastVisitAt, sec.HasVisit = snapshot.AsInt(m["lastVisitAt"])
	if partial := snapshot.AsMap(m["lastSearchPartial"]); partial != nil {
		sec.Partial = &SearchPartial{Total: partial["total"], Shown: partial["shown"]}
	}
	return sec
}

func readPrice(key string, raw any) Price {
	m := snapshot.AsMap(raw)
	p := Price{Key: key, Name: m["name"], Raw: raw}
	p.Passed = present(m, passedKeys)
	p.Details = present(m, detailKeys)
	p.MinBuyout, p.HasBuyout = snapshot.AsInt(m["minBuyout"])
	p.MinBid, p.HasBid = snapshot.AsInt(m["minBid"])
	p.SeenAt, p.HasSeenAt = snapshot.AsInt(m["seenAt"])
	for _, visit := range snapshot.AsList(m["seen"]) {
		buyout, _ := snapshot.AsInt(snapshot.AsMap(visit)["minBuyout"])
		p.Buyouts = append(p.Buyouts, buyout)
	}
	_, equip := m["equipLoc"]
	_, class := m["classID"]
	p.HasDetails = equip || class
	_, p.HasRequiredLevel = m["requiredLevel"]
	_, level := m["itemLevel"]
	_, stats := m["statTokens"]
	p.HasNumbers = level || stats
	return p
}

func present(m map[string]any, keys []string) []Detail {
	var out []Detail
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out = append(out, Detail{k, v})
		}
	}
	return out
}

func readOwnAuction(m map[string]any) OwnAuction {
	a := OwnAuction{Name: m["name"], Count: m["count"], Units: snapshot.ItemCount(m), Sold: m["sold"] == true}
	if v, ok := snapshot.AsInt(m["buyout"]); ok && v > 0 {
		a.Buyout = v
	}
	if v, ok := snapshot.AsInt(m["bid"]); ok && v > 0 {
		a.Bid = v
	}
	a.TimeLeft, a.HasTimeLeft = snapshot.AsInt(m["timeLeftSeconds"])
	return a
}

// readOwnedItems sums the items of bags and bank by name.
func readOwnedItems(s snapshot.Snapshot) map[string]*OwnedItem {
	out := map[string]*OwnedItem{}
	add := func(inBank bool, list []any) {
		for _, raw := range list {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			o := out[name]
			if o == nil {
				o = &OwnedItem{Name: name}
				o.SellPrice, _ = snapshot.AsInt(item["sellPrice"])
				o.Quality, _ = snapshot.AsInt(item["quality"])
				out[name] = o
			}
			kind := itemkind.Read(item)
			o.Count += snapshot.ItemCount(item)
			o.InBags = o.InBags || !inBank
			o.InBank = o.InBank || inBank
			o.Quest = o.Quest || kind.IsQuestItem()
			o.Bound = o.Bound || kind.IsBound()
		}
	}
	for _, bag := range s.Bags() {
		add(false, snapshot.AsList(snapshot.AsMap(bag)["items"]))
	}
	for _, c := range s.BankContainers() {
		add(true, snapshot.AsList(snapshot.AsMap(c)["items"]))
	}
	return out
}
