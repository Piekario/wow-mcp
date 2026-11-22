// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package loot reads the loot section of a dump: the player's own loot collected by the addon since the previous write to disk
// (from chat messages about their own loot, never anyone else's), plus the periods of earlier saves it carries (addon 0.12.0). The session journal and wow_changes show the interesting parts of it.
package loot

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/periods"
	"wowmcp/internal/snapshot"
)

// NotableQuality: loot of this quality (Uncommon) and above is "notable".
const NotableQuality = 2

// UnknownQuality marks loot for which the addon gave no quality.
const UnknownQuality = -1

var qualityNames = []string{"Poor", "Common", "Uncommon", "Rare", "Epic", "Legendary"}

// QualityLabel returns the in-game quality name; "?" for an unknown one.
func QualityLabel(q int64) string {
	if q >= 0 && int(q) < len(qualityNames) {
		return qualityNames[q]
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

// Section returns the items from the loot section of one dump and the time from which the loot counts (since);
// false when the dump has no loot section (older addon or the section disabled). Only the current period:
// periods carried from earlier saves are merged by Gained.
func Section(s snapshot.Snapshot) (items []Item, since int64, ok bool) {
	sec := s.Map("loot")
	if sec == nil {
		return nil, 0, false
	}
	since, _ = snapshot.AsInt(sec["since"])
	return itemsOf(sec), since, true
}

// itemsOf reads the items of one loot period (the section or a carried period).
func itemsOf(period map[string]any) (items []Item) {
	for _, raw := range snapshot.AsList(period["items"]) {
		m := snapshot.AsMap(raw)
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		it := Item{Name: name, Quality: UnknownQuality, Count: snapshot.ItemCount(m)}
		it.ID, _ = snapshot.AsInt(m["id"])
		if q, ok := snapshot.AsInt(m["quality"]); ok {
			it.Quality = q
		}
		items = append(items, it)
	}
	return items
}

// Gained sums the loot gained after baseline (nil: no baseline) from the dumps: their current periods and the periods
// carried from saves the server never read (periods.After). The merge says how many carried periods it used.
func Gained(baseline snapshot.Snapshot, dumps []snapshot.Snapshot) ([]Item, periods.Merge) {
	m := periods.After(baseline, dumps, "loot")
	lists := make([][]Item, len(m.Periods))
	for i, p := range m.Periods {
		lists[i] = itemsOf(p.Fields)
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

// Total is the total number of units.
func Total(items []Item) int64 {
	var n int64
	for _, it := range items {
		n += it.Count
	}
	return n
}

// Describe describes loot for the response: counts, notable entries (up to maxListed) and a note about unknown quality.
// An empty map (nil) when there was no loot at all.
func Describe(items []Item, maxListed int) map[string]any {
	if len(items) == 0 {
		return nil
	}
	notable, unknown := Notable(items)
	out := map[string]any{"distinctItems": len(items), "pieces": Total(items)}
	if len(notable) > 0 {
		shown := notable
		if len(shown) > maxListed {
			shown = shown[:maxListed]
		}
		labels := make([]string, len(shown))
		for i, it := range shown {
			labels[i] = it.Label()
		}
		out["notable"] = labels
		if more := len(notable) - len(shown); more > 0 {
			out["more"] = more
		}
	}
	if unknown > 0 {
		out["unknownQuality"] = unknown
		out["qualityNote"] = fmt.Sprintf("the addon gave no quality for %d items, so they are not included in the notable list", unknown)
	}
	return out
}
