// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// Holdings is how many units of one item a character has: in bags, in the bank (state from the last visit) and equipped.
type Holdings struct {
	Bags, Bank, Equipped int64
}

func (h Holdings) Total() int64 { return h.Bags + h.Bank + h.Equipped }

// Count counts units of the item with the exact name (case-insensitive).
func Count(s snapshot.Snapshot, name string) Holdings {
	var h Holdings
	units := func(list []Item) int64 {
		var n int64
		for _, it := range list {
			if strings.EqualFold(it.Name, name) {
				n += it.Count
			}
		}
		return n
	}
	for _, bag := range readBags(s) {
		h.Bags += units(bag.Items)
	}
	bank, _ := readBank(s)
	for _, c := range bank.Containers {
		h.Bank += units(c.Items)
	}
	h.Equipped = units(readEquipment(s))
	return h
}
