// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// StockLine is the stock of one item summed by name across bags and bank (bank: state from the last visit).
type StockLine struct {
	Name, Type, SubType string
	Item                map[string]any // the first instance found (class IDs, binding)
	Bags, Bank          int64
}

func (l StockLine) Total() int64 { return l.Bags + l.Bank }

// BagBankStock sums items from bags and bank by name (key: lowercase name).
// keep allows skipping items (e.g. bound ones); nil = takes all. It does not count equipped gear.
func BagBankStock(s snapshot.Snapshot, keep func(item map[string]any) bool) map[string]*StockLine {
	out := map[string]*StockLine{}
	add := func(bank bool, list []any) {
		for _, raw := range list {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" || (keep != nil && !keep(item)) {
				continue
			}
			key := strings.ToLower(name)
			l := out[key]
			if l == nil {
				l = &StockLine{Name: name, Item: item}
				l.Type, _ = item["type"].(string)
				l.SubType, _ = item["subType"].(string)
				out[key] = l
			}
			if bank {
				l.Bank += snapshot.ItemCount(item)
			} else {
				l.Bags += snapshot.ItemCount(item)
			}
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
