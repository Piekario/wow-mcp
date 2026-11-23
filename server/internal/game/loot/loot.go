// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package loot reads the loot section of a dump: the player's own loot collected by the addon since the previous write to disk
// (from chat messages about their own loot, never anyone else's), plus the periods of earlier saves it carries (addon 0.12.0). The session journal and wow_changes show the interesting parts of it.
package loot

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/periods"
	"wowmcp/internal/snapshot"
)

// NotableQuality: loot of this quality (Uncommon) and above is "notable".
const NotableQuality = 2

// UnknownQuality marks loot for which the addon gave no quality.
const UnknownQuality = -1

// QualityLabel returns the in-game quality name; "?" for an unknown one.
func QualityLabel(q int64) string {
	if name, ok := itemkind.QualityName(q); ok {
		return name
	}
	return "?"
}

// Item is an item obtained as loot, summed by name.
type Item struct {
	Name    string
	ID      int64
	Quality int64 // UnknownQuality when the addon gave none
	Count   int64
}

// Label writes the item as in responses: "Name xN (Quality)".
func (it Item) Label() string {
	return fmt.Sprintf("%s x%d (%s)", it.Name, it.Count, QualityLabel(it.Quality))
}

// Gained sums the loot gained after baseline (nil: no baseline) from the dumps: their current periods and the periods
// carried from saves the server never read (periods.After). The merge says how many carried periods it used.
func Gained(baseline snapshot.Snapshot, dumps []snapshot.Snapshot) ([]Item, periods.Merge) {
	m := periods.After(baseline, dumps, "loot")
	lists := make([][]Item, len(m.Periods))
	for i, p := range m.Periods {
		lists[i] = itemsOf(p.Raw)
	}
	return sum(lists), m
}

func sum(lists [][]Item) []Item {
	byName := map[string]*Item{}
	for _, items := range lists {
		for _, it := range items {
			e := byName[it.Name]
			if e == nil {
				cp := it
				cp.Count = 0
				e = &cp
				byName[it.Name] = e
			}
			e.Count += it.Count
			e.Quality = max(e.Quality, it.Quality)
		}
	}
	out := make([]Item, 0, len(byName))
	for _, it := range byName {
		out = append(out, *it)
	}
	Sort(out)
	return out
}

// Sort orders loot by best quality, then alphabetically.
func Sort(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Quality != items[j].Quality {
			return items[i].Quality > items[j].Quality
		}
		return items[i].Name < items[j].Name
	})
}

// Summary is the loot of a period boiled down: how much, the notable entries (up to the limit) and how many were cut.
type Summary struct {
	Distinct       int
	Pieces         int64
	Notable        []Item
	More           int
	UnknownQuality int
}

// Summarise returns nil when there was no loot at all.
func Summarise(items []Item, maxListed int) *Summary {
	if len(items) == 0 {
		return nil
	}
	notable, unknown := Notable(items)
	sum := &Summary{Distinct: len(items), Pieces: Total(items), Notable: notable, UnknownQuality: unknown}
	if len(notable) > maxListed {
		sum.More = len(notable) - maxListed
		sum.Notable = notable[:maxListed]
	}
	return sum
}

// Notable picks the notable loot (Uncommon quality and above) and says how many entries have no known quality.
func Notable(items []Item) (notable []Item, unknownQuality int) {
	for _, it := range items {
		switch {
		case it.Quality == UnknownQuality:
			unknownQuality++
		case it.Quality >= NotableQuality:
			notable = append(notable, it)
		}
	}
	return notable, unknownQuality
}

func Total(items []Item) int64 {
	var n int64
	for _, it := range items {
		n += it.Count
	}
	return n
}
