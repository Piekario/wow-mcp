// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package bagplan

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/leftovers"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

type bagContents struct {
	Stacks       []Stack
	Space        items.BagSlots
	BindingKnown bool
}

// readBags sums the bags by name, in order of first appearance. The typed first instance comes from items.StockOf,
// which has no bag order and no slot count.
func readBags(s snapshot.Snapshot) bagContents {
	stock := items.StockOf(s, nil)
	out := bagContents{Space: items.Space(s)}
	byName := map[string]*Stack{}
	var order []*Stack
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			if itemkind.BindingKnown(item) {
				out.BindingKnown = true
			}
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			st := byName[name]
			if st == nil {
				st = &Stack{Name: name}
				if l := stock[strings.ToLower(name)]; l != nil {
					st.Item = l.Item
				}
				st.Sell = st.Item.SellPrice
				byName[name] = st
				order = append(order, st)
			}
			n := snapshot.ItemCount(item)
			st.Count += n
			st.Slots++
			st.Vendor += st.Sell * n
		}
	}
	for _, st := range order {
		out.Stacks = append(out.Stacks, *st)
	}
	return out
}

func readBank(s snapshot.Snapshot) BankSpace {
	bank := s.Map("bank")
	if bank == nil {
		return BankSpace{}
	}
	b := BankSpace{Saved: true}
	for _, c := range snapshot.AsList(bank["containers"]) {
		n, _ := snapshot.AsInt(snapshot.AsMap(c)["free"])
		b.FreeSlots += n
	}
	return b
}

func readContext(s snapshot.Snapshot, fresh snapshot.Freshness) *context {
	c := &context{
		buyer: items.NewBuyer(s), questKeep: map[string]string{}, questOther: map[string]bool{},
		reagents: professions.KnownReagents(s), offers: auctions.Offers(s, fresh),
		skills: professions.Known(s), hasPet: pet.Exists(s),
	}
	for _, it := range c.buyer.Equipped {
		if it.HasSlot && it.Slot == "ranged" {
			c.rangedKind = it.RangedAmmo()
		}
	}
	for _, d := range snapshot.AsList(s.Map("pet")["foodTypes"]) {
		c.diet = append(c.diet, fmt.Sprint(d))
	}
	verdicts, _ := leftovers.Assess(s)
	for _, v := range verdicts {
		if v.Keep {
			c.questKeep[v.Name] = v.Reason
		} else {
			c.questOther[v.Name] = true
		}
	}
	return c
}
