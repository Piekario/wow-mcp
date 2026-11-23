// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package makeorbuy

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// price is a known price per unit.
type price struct {
	copper    int64
	source    string
	seenAt    int64 // time the auction price was saved
	hasSeenAt bool
}

func (p price) quote(fresh snapshot.Freshness) Quote {
	q := Quote{Copper: p.copper, Source: p.source}
	if p.hasSeenAt {
		q.Age, q.HasAge = fresh.Age(p.seenAt), true
	}
	return q
}

// priceBook holds auction prices (by name and ID) and vendor prices (by name, lowercase).
type priceBook struct {
	auctionByName map[string]price
	auctionByID   map[int64]price
	vendor        map[string]price
}

func newBook(m market) priceBook {
	b := priceBook{auctionByName: map[string]price{}, auctionByID: map[int64]price{}, vendor: map[string]price{}}
	for _, q := range m.Auction {
		p := price{copper: q.Buyout, source: "auction", seenAt: q.SeenAt, hasSeenAt: q.HasSeenAt}
		b.auctionByName[strings.ToLower(q.Name)] = p
		if q.HasItemID {
			b.auctionByID[q.ItemID] = p
		}
	}
	for _, o := range m.Vendor {
		stack := max(o.Stack, 1)
		key := strings.ToLower(o.Name)
		unit := (o.Total + stack - 1) / stack
		if old, seen := b.vendor[key]; !seen || unit < old.copper {
			b.vendor[key] = price{copper: unit, source: "vendor:" + o.Npc}
		}
	}
	return b
}

// reagentPrice picks the cheaper of the known reagent prices.
func (b priceBook) reagentPrice(name string) (price, bool) {
	key := strings.ToLower(name)
	a, hasA := b.auctionByName[key]
	v, hasV := b.vendor[key]
	switch {
	case hasA && hasV:
		if v.copper <= a.copper {
			return v, true
		}
		return a, true
	case hasA:
		return a, true
	case hasV:
		return v, true
	}
	return price{}, false
}

// productPrice is the auction price of the finished item (by recipe name or item ID).
func (b priceBook) productPrice(name string, itemID int64) (price, bool) {
	if p, ok := b.auctionByName[strings.ToLower(name)]; ok {
		return p, true
	}
	if itemID > 0 {
		p, ok := b.auctionByID[itemID]
		return p, ok
	}
	return price{}, false
}
