// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// Places an item can be found in.
const (
	InBags     = "bags"
	InBank     = "bank"
	AtVendor   = "vendor"
	InMail     = "mail"
	IsEquipped = "equipped"
)

// FoundItem is an item found on a character and the place it comes from (bags, bank, vendor).
type FoundItem struct {
	Item   Item
	Source string
}

// FindItems looks for an item in bags, bank and at a vendor; source narrows it to one place.
// An empty name matches everything. An exact name beats a fragment.
func FindItems(s snapshot.Snapshot, query, source string) []FoundItem {
	match := matcher(query)
	var found []FoundItem
	add := func(src string, list []Item) {
		if source != "" && !strings.EqualFold(source, src) {
			return
		}
		for _, it := range list {
			if match(it) {
				found = append(found, FoundItem{it, src})
			}
		}
	}
	for _, bag := range readBags(s) {
		add(InBags, bag.Items)
	}
	bank, _ := readBank(s)
	for _, c := range bank.Containers {
		add(InBank, c.Items)
	}
	vendor, _ := readLastVendor(s)
	add(AtVendor, vendor.Items)

	var exact []FoundItem
	for _, f := range found {
		if strings.EqualFold(f.Item.Name, query) {
			exact = append(exact, f)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return found
}

// FindQuery is the input of wow_find_item.
type FindQuery struct {
	Item      string `json:"item" jsonschema:"item name (or a fragment of it) or its ID"`
	Character string `json:"character,omitempty" jsonschema:"only this character; empty = all characters on the account"`
}

// Location is one place of an item on a character. Container, From and Subject are passed through from the dump;
// SeenAt is the time the bank was saved.
type Location struct {
	Character     string
	Where         string
	Item          Item
	Container     any
	SeenAt        snapshot.Captured
	From, Subject any
}

// AccountSearch is an item searched for across the account, with a total for each item name.
type AccountSearch struct {
	Query      string
	Characters []string
	Locations  []Location
	Totals     map[string]int64
	Total      int64
}

// SearchAccount searches for an item across all characters on the account at once: bags, equipped gear, bank and mail.
func SearchAccount(db *snapshot.Database, q FindQuery) (AccountSearch, error) {
	if strings.TrimSpace(q.Item) == "" {
		return AccountSearch{}, fmt.Errorf("give an item name or ID in the item parameter")
	}
	names := db.CharacterNames()
	if q.Character != "" {
		if _, _, err := db.Lookup(q.Character); err != nil {
			return AccountSearch{}, err
		}
		names = []string{q.Character}
	}
	match := matcher(q.Item)
	r := AccountSearch{Query: q.Item, Characters: names, Totals: map[string]int64{}}
	for _, name := range names {
		for _, loc := range locations(db.Characters[name], match) {
			loc.Character = name
			r.Totals[loc.Item.nameLabel()] += loc.Item.Count
			r.Total += loc.Item.Count
			r.Locations = append(r.Locations, loc)
		}
	}
	return r, nil
}

func locations(s snapshot.Snapshot, match func(Item) bool) []Location {
	var out []Location
	for _, bag := range readBags(s) {
		for _, it := range bag.Items {
			if match(it) {
				out = append(out, Location{Where: InBags, Item: it, Container: bag.Name})
			}
		}
	}
	bank, _ := readBank(s)
	for _, c := range bank.Containers {
		for _, it := range c.Items {
			if match(it) {
				out = append(out, Location{Where: InBank, Item: it, Container: c.Name, SeenAt: bank.Captured})
			}
		}
	}
	for _, it := range readEquipment(s) {
		if match(it) {
			out = append(out, Location{Where: IsEquipped, Item: it})
		}
	}
	for _, letter := range readMailItems(s) {
		for _, it := range letter.Items {
			if match(it) {
				out = append(out, Location{Where: InMail, Item: it, From: letter.Sender, Subject: letter.Subject})
			}
		}
	}
	return out
}

// Letter is a letter in the mailbox with its attached items, as far as finding items needs it.
type Letter struct {
	Sender, Subject any
	Items           []Item
}
