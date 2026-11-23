// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "wowmcp/internal/snapshot"

// BankQuery holds the bank content filters (wow_bank).
type BankQuery struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of an item name"`
	Type       string `json:"type,omitempty" jsonschema:"item type or subtype, e.g. Weapon, Armor, Trade Goods, Cloth"`
	MinQuality int64  `json:"minQuality,omitempty" jsonschema:"minimum quality: 0 Poor, 1 Common, 2 Uncommon, 3 Rare, 4 Epic"`
	Sellable   bool   `json:"sellable,omitempty" jsonschema:"true = only items a vendor will buy (sell price > 0)"`
}

// BankSection is the bank from the last visit.
type BankSection struct {
	Containers []Container
	Captured   snapshot.Captured
}

type BankEntry struct {
	Item
	Container any
}

// BankReport is the bank filtered by a query. Saved is false when the player never opened the bank.
type BankReport struct {
	Saved                 bool
	Entries               []BankEntry
	TotalSlots, FreeSlots int64
	TotalValue            int64
	Captured              snapshot.Captured
}

func InspectBank(s snapshot.Snapshot, q BankQuery) BankReport {
	bank, ok := readBank(s)
	if !ok {
		return BankReport{}
	}
	f := filter{query: q.Query, kind: q.Type, minQuality: q.MinQuality}
	slots := slotsOf(bank.Containers)
	r := BankReport{Saved: true, TotalSlots: slots.Total, FreeSlots: slots.Free, Captured: bank.Captured}
	for _, c := range bank.Containers {
		for _, it := range c.Items {
			if !f.matches(it) || (q.Sellable && it.SellPrice <= 0) {
				continue
			}
			if it.SellPrice > 0 {
				r.TotalValue += it.SellValue()
			}
			r.Entries = append(r.Entries, BankEntry{Item: it, Container: c.Name})
		}
	}
	return r
}
