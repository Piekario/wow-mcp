// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// ItemStock is the stock of one item summed by name across bags and bank (bank: state from the last visit).
// Item is the first instance found (class IDs, binding).
type ItemStock struct {
	Item       Item
	Bags, Bank int64
}

func (l ItemStock) Total() int64 { return l.Bags + l.Bank }

// StockOf sums items from bags and bank by name (key: lowercase name). keep allows skipping items
// (e.g. bound ones); nil takes all. It does not count equipped gear.
func StockOf(s snapshot.Snapshot, keep func(Item) bool) map[string]*ItemStock {
	out := map[string]*ItemStock{}
	add := func(bank bool, list []Item) {
		for _, it := range list {
			if it.Name == "" || (keep != nil && !keep(it)) {
				continue
			}
			key := strings.ToLower(it.Name)
			l := out[key]
			if l == nil {
				l = &ItemStock{Item: it}
				out[key] = l
			}
			if bank {
				l.Bank += it.Count
			} else {
				l.Bags += it.Count
			}
		}
	}
	for _, bag := range readBags(s) {
		add(false, bag.Items)
	}
	bank, _ := readBank(s)
	for _, c := range bank.Containers {
		add(true, c.Items)
	}
	return out
}
