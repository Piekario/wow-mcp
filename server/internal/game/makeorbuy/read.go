// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package makeorbuy

import (
	"maps"
	"slices"

	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// auctionQuote is a saved auction house price: the lowest buyout of one unit.
type auctionQuote struct {
	Name      string
	ItemID    int64
	HasItemID bool
	Buyout    int64
	SeenAt    int64
	HasSeenAt bool
}

// vendorOffer is an item a remembered vendor sells: Total is the price of Stack units.
type vendorOffer struct {
	Npc          string
	Name         string
	Total, Stack int64
}

// market is every price the dump knows.
type market struct {
	Auction     []auctionQuote
	Vendor      []vendorOffer
	LastVisitAt int64
	HasVisit    bool
}

func readMarket(s snapshot.Snapshot) market {
	var m market
	prices := snapshot.AsMap(s.Map("auctions")["prices"])
	for _, name := range slices.Sorted(maps.Keys(prices)) {
		p := snapshot.AsMap(prices[name])
		buyout, ok := snapshot.AsInt(p["minBuyout"])
		if !ok || buyout <= 0 {
			continue
		}
		q := auctionQuote{Name: name, Buyout: buyout}
		q.SeenAt, q.HasSeenAt = snapshot.AsInt(p["seenAt"])
		q.ItemID, q.HasItemID = snapshot.AsInt(p["id"])
		m.Auction = append(m.Auction, q)
	}
	m.LastVisitAt, m.HasVisit = snapshot.AsInt(s.Map("auctions")["lastVisitAt"])
	for _, v := range s.Visits("vendor", "vendors") {
		for _, raw := range snapshot.AsList(v.Section["items"]) {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			total, ok := snapshot.AsInt(item["priceCopper"])
			if name == "" || !ok || total <= 0 {
				continue
			}
			stack, _ := snapshot.AsInt(item["stack"])
			m.Vendor = append(m.Vendor, vendorOffer{Npc: v.Npc, Name: name, Total: total, Stack: stack})
		}
	}
	return m
}

// readCrafts lists the saved recipes that have reagents and a name, for the professions and names asked about.
func readCrafts(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) (crafts []Craft, saved bool) {
	report := professions.AnalyseRecipes(s, professions.RecipeQuery{Profession: q.Profession, Query: q.Query}, fresh)
	for _, book := range report.Books {
		for _, status := range book.Recipes {
			r := status.Recipe
			if len(r.Reagents) == 0 || r.Name == "" {
				continue
			}
			c := Craft{Profession: book.Name, Name: r.Name}
			c.ItemID, _ = snapshot.AsInt(snapshot.AsMap(r.Raw)["itemId"])
			for _, re := range r.Reagents {
				c.Reagents = append(c.Reagents, Reagent{Name: re.Name, Need: max(re.Need, 1)})
			}
			crafts = append(crafts, c)
		}
	}
	return crafts, report.Saved
}
