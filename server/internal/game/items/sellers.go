// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Seller is a remembered vendor that had the given item on offer.
type Seller struct {
	Npc         string
	PriceCopper int64   // price per pack (a stack of units), as the vendor window shows it
	Stack       int64   // units per pack (1 when the offer does not say)
	UnitCopper  float64 // price of one unit
	CapturedAt  int64
}

// SellersOf looks among the remembered vendors (vendor and vendors) for an item with this name
// (case-insensitive) and returns them from the cheapest unit. The offer may be old: see CapturedAt.
func SellersOf(s snapshot.Snapshot, name string) []Seller {
	var out []Seller
	for _, v := range s.Visits("vendor", "vendors") {
		vendor, _ := readVendorSection(v.Section)
		for _, it := range vendor.Items {
			if !strings.EqualFold(it.Name, name) || !it.HasPrice {
				continue
			}
			stack := max(it.Stack, 1)
			at, _ := v.CapturedAt()
			out = append(out, Seller{Npc: v.Npc, PriceCopper: it.Price, Stack: stack, UnitCopper: float64(it.Price) / float64(stack), CapturedAt: at})
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UnitCopper < out[j].UnitCopper })
	return out
}

// CostFor computes the cost of buying at least n units in whole packs: how many packs and what they cost.
func (sl Seller) CostFor(n int64) (packs, copper int64) {
	if n <= 0 {
		return 0, 0
	}
	packs = (n + sl.Stack - 1) / sl.Stack
	return packs, packs * sl.PriceCopper
}
