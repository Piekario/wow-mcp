// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package ammo covers ammunition (arrows and bullets): how much is in the equipped slot and in bags,
// how much is used per hour of play (from dump history) and how much to buy before heading out.
package ammo

import (
	"fmt"
	"math"
	"sort"

	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_ammo.
type Query struct {
	Character string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Hours     float64 `json:"hours,omitempty" jsonschema:"how many hours of play to plan the ammo purchase for; default 2"`
}

// defaultHours is the default play time the ammo purchase is planned for.
const defaultHours = 2

// Stock is the supply of one kind of ammo.
type Stock struct {
	Name     string
	Equipped int64 // in the ammo slot
	InBags   int64
}

func (s Stock) Total() int64 { return s.Equipped + s.InBags }

// isAmmoItem: ammo is an item of the Projectile class (for old dumps: type Projectile or subtype Arrow/Bullet).
func isAmmoItem(item map[string]any) bool { return itemkind.IsAmmo(item) }

// equippedAmmo returns the item in the ammo slot; nil when the slot is empty.
func equippedAmmo(s snapshot.Snapshot) map[string]any {
	return snapshot.EquippedBySlot(s.Equipment())["ammo"]
}

// bagAmmo counts ammo in bags by name.
func bagAmmo(s snapshot.Snapshot) map[string]int64 {
	out := map[string]int64{}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			if item := snapshot.AsMap(raw); isAmmoItem(item) {
				out[fmt.Sprint(item["name"])] += snapshot.ItemCount(item)
			}
		}
	}
	return out
}

// MainStock picks the main ammo kind: the equipped one, or the most numerous in bags when the slot is empty.
// The second result is the other kinds from bags. false when the character has no ammo at all.
func MainStock(s snapshot.Snapshot) (main Stock, others []Stock, ok bool) {
	inBags := bagAmmo(s)
	if eq := equippedAmmo(s); eq != nil {
		main = Stock{Name: fmt.Sprint(eq["name"]), Equipped: snapshot.ItemCount(eq)}
	} else if name, found := mostNumerous(inBags); found {
		main = Stock{Name: name}
	} else {
		return Stock{}, nil, false
	}
	main.InBags = inBags[main.Name]
	delete(inBags, main.Name)
	for name, n := range inBags {
		others = append(others, Stock{Name: name, InBags: n})
	}
	sort.Slice(others, func(i, j int) bool { return others[i].Name < others[j].Name })
	return main, others, true
}

func mostNumerous(counts map[string]int64) (string, bool) {
	best, found := "", false
	for name, n := range counts {
		if !found || n > counts[best] || (n == counts[best] && name < best) {
			best, found = name, true
		}
	}
	return best, found
}

// countOf counts ammo of a given kind in the dump (ammo slot and bags).
func countOf(s snapshot.Snapshot, name string) int64 {
	var n int64
	if eq := equippedAmmo(s); eq != nil && fmt.Sprint(eq["name"]) == name {
		n += snapshot.ItemCount(eq)
	}
	return n + bagAmmo(s)[name]
}

// View describes the ammo stock, the per-hour usage from history and how much to buy for q.Hours hours.
func View(name string, s snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	hours := q.Hours
	switch {
	case hours == 0:
		hours = defaultHours
	case hours < 0 || math.IsNaN(hours):
		return nil, fmt.Errorf("hours must be positive (default %d), got %g", defaultHours, q.Hours)
	}
	main, others, ok := MainStock(s)
	if !ok {
		return map[string]any{"note": "The character has no ammo: the ammo slot is empty and there are no arrows or bullets in bags. " +
			"Ammo applies to characters with a ranged weapon that uses it (e.g. Hunter); if you have some, open your bags and run /mcp reload."}, nil
	}
	out := map[string]any{"ammo": stockView(main)}
	if len(others) > 0 {
		views := make([]any, len(others))
		for i, o := range others {
			views[i] = stockView(o)
		}
		out["otherAmmoInBags"] = views
	}
	if equippedAmmo(s) == nil {
		out["equippedNote"] = "The ammo slot is empty; calculated from the most numerous ammo in bags."
	}

	use, err := consumption.Over(tr, name, s, func(snap snapshot.Snapshot) int64 { return countOf(snap, main.Name) })
	if err != nil {
		out["consumption"] = map[string]any{"note": err.Error()}
		return out, nil
	}
	out["consumption"] = use.Describe(consumptionHow, notShrunk)
	if left, ok := use.HoursLeft(main.Total()); ok {
		out["hoursLeft"] = snapshot.Round1(left)
		out["timeLeft"] = snapshot.FormatSeconds(int64(left * 3600))
		out["plan"] = purchasePlan(main, use.PerHour, hours, s)
	}
	return out, nil
}

func stockView(st Stock) map[string]any {
	return map[string]any{"name": st.Name, "equipped": st.Equipped, "inBags": st.InBags, "total": st.Total()}
}

// consumptionHow explains how ammo usage is calculated.
const consumptionHow = "sum of drops in the count between consecutive dumps (purchases and increases do not count); this is an approximation, because the loss can have causes other than shooting"

// notShrunk is the note used when the ammo did not decrease within the history stretch.
const notShrunk = "The ammo did not decrease in this stretch of history, so there is nothing to compute a rate from."

// purchasePlan computes how much to buy to last hours hours of play; adds the stack size if the last vendor sells this ammo.
func purchasePlan(main Stock, perHour, hours float64, s snapshot.Snapshot) map[string]any {
	needed := int64(math.Ceil(perHour * hours))
	toBuy := max(needed-main.Total(), 0)
	out := map[string]any{"hours": hours, "needed": needed, "have": main.Total(), "toBuy": toBuy}
	if toBuy == 0 {
		out["note"] = "The stock will last for the planned play time."
	}
	if vendor := s.Map("vendor"); vendor != nil {
		for _, raw := range snapshot.AsList(vendor["items"]) {
			if item := snapshot.AsMap(raw); item["name"] == main.Name {
				out["vendor"] = map[string]any{"npc": vendor["npc"], "stack": item["stack"], "note": "offer from the last vendor visit"}
			}
		}
	}
	return out
}

// Forecast is the ammo supply forecast: how much there is and, when history allows, how many hours of play it lasts.
type Forecast struct {
	Present   bool // the character has any ammo at all
	Stock     Stock
	RateKnown bool // the hourly usage is known
	PerHour   float64
	HoursLeft float64 // meaningful only when RateKnown and usage is above zero
	Burning   bool    // ammo is actually being used up (PerHour > 0)
	Note      string  // why there is no rate (history too short, history disabled)
}

// Left computes the forecast of the main ammo stock from dump history (the same window as wow_ammo).
func Left(name string, s snapshot.Snapshot, tr progress.Tracker) Forecast {
	main, _, ok := MainStock(s)
	if !ok {
		return Forecast{}
	}
	f := Forecast{Present: true, Stock: main}
	use, err := consumption.Over(tr, name, s, func(snap snapshot.Snapshot) int64 { return countOf(snap, main.Name) })
	if err != nil {
		f.Note = err.Error()
		return f
	}
	f.RateKnown, f.PerHour = true, use.PerHour
	if left, ok := use.HoursLeft(main.Total()); ok {
		f.Burning, f.HoursLeft = true, left
	}
	return f
}
