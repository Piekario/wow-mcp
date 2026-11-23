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
	same := func(item map[string]any) bool {
		n, _ := item["name"].(string)
		return strings.EqualFold(n, name)
	}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			if item := snapshot.AsMap(raw); same(item) {
				h.Bags += snapshot.ItemCount(item)
			}
		}
	}
	for _, c := range s.BankContainers() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(c)["items"]) {
			if item := snapshot.AsMap(raw); same(item) {
				h.Bank += snapshot.ItemCount(item)
			}
		}
	}
	for _, raw := range s.Equipment() {
		if item := snapshot.AsMap(raw); same(item) {
			h.Equipped += snapshot.ItemCount(item)
		}
	}
	return h
}
