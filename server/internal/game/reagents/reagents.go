// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package reagents covers stocks of things that get used up: ammo, food and water, bandages, potions,
// other consumables and class reagents. For each it gives how many there are now and, when history allows,
// how many go per hour of play and for how many hours the stock lasts. Consumption is computed by the consumption package, shared with ammo.
package reagents

import (
	"sort"

	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// category is a group of consumable things.
type category struct {
	key, label string
}

// Groups in response order.
var (
	ammo      = category{"ammo", "Ammo"}
	food      = category{"food", "Food and water"}
	bandages  = category{"bandages", "Bandages"}
	potions   = category{"potions", "Potions, elixirs and flasks"}
	consumes  = category{"otherConsumables", "Other consumables"}
	reagent   = category{"reagents", "Reagents"}
	orderedBy = []category{ammo, food, bandages, potions, consumes, reagent}
)

// classify assigns an item to a group by its item class and subclass (for old dumps: the English type and
// subtype); false when it is not consumable.
func classify(item itemkind.Item) (category, bool) {
	switch {
	case item.IsAmmo():
		return ammo, true
	case item.Is(itemkind.ClassReagent):
		return reagent, true
	}
	switch item.ConsumableKind() {
	case itemkind.Food:
		return food, true
	case itemkind.Bandage:
		return bandages, true
	case itemkind.Potion:
		return potions, true
	case itemkind.Other:
		return consumes, true
	}
	return category{}, false
}

// stock is one consumable item of the character.
type stock struct {
	name string
	cat  category
}

// collect gathers consumable items from bags and equipped ammo, in a stable name order.
func collect(s snapshot.Snapshot) []stock {
	seen := map[string]category{}
	for _, item := range readCarried(s) {
		if cat, ok := classify(item); ok && item.Name != "" {
			seen[item.Name] = cat
		}
	}
	list := make([]stock, 0, len(seen))
	for name, cat := range seen {
		list = append(list, stock{name, cat})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}

// lowStockHours: with this many hours of play of stock the item goes into lowStock.
const lowStockHours = 1.0

// Line is one consumable: what the character holds and, when history allows, how fast it goes.
type Line struct {
	Name         string
	Holdings     items.Holdings
	Measured     bool
	Use          consumption.Usage
	HoursLeft    float64
	HasHoursLeft bool
}

// Total is the stock that counts for consumption: bags plus what the character wears. The bank is not included.
func (l Line) Total() int64 { return l.Holdings.Bags + l.Holdings.Equipped }

// Low: the stock lasts less than an hour of play at the measured rate.
func (l Line) Low() bool { return l.HasHoursLeft && l.HoursLeft < lowStockHours }

type Group struct {
	category
	Lines []Line
}

func (g Group) IsAmmo() bool { return g.category == ammo }

// PetFood is the food in bags next to the pet's diet.
type PetFood struct {
	FoodTypes any
	Checked   pet.FoodCheck   // the client's verdict on the bag food (addon 0.22.0)
	InBags    []pet.FoodStack // food by item subtype, only when the verdict is not in the dump
}

// Report is the stocks of consumable items by category and their consumption from history. WindowErr says why
// there is no consumption window (history disabled or too short).
type Report struct {
	Groups    []Group
	Window    consumption.Usage
	HasWindow bool
	WindowErr string
	PetFood   *PetFood
}

func (r Report) Empty() bool { return len(r.Groups) == 0 }

// Inspect measures the stocks of consumable items and their consumption from history.
func Inspect(name string, s snapshot.Snapshot, tr progress.Tracker) Report {
	stocks := collect(s)
	if len(stocks) == 0 {
		return Report{}
	}
	windows, werr := consumption.Load(tr, name, s)
	var r Report
	var measureErr string
	byCat := map[category][]Line{}
	for _, st := range stocks {
		line := Line{Name: st.name, Holdings: items.Count(s, st.name)}
		if werr == nil {
			use, err := windows.Measure(func(snap snapshot.Snapshot) int64 {
				c := items.Count(snap, st.name)
				return c.Bags + c.Equipped
			})
			if err != nil {
				measureErr = err.Error()
			} else {
				line.Measured, line.Use = true, use
				line.HoursLeft, line.HasHoursLeft = use.HoursLeft(line.Total())
				if !r.HasWindow {
					r.Window, r.HasWindow = use, true
				}
			}
		}
		byCat[st.cat] = append(byCat[st.cat], line)
	}
	for _, cat := range orderedBy {
		if lines := byCat[cat]; len(lines) > 0 {
			r.Groups = append(r.Groups, Group{category: cat, Lines: lines})
		}
	}
	switch {
	case werr != nil:
		r.WindowErr = werr.Error()
	case !r.HasWindow:
		r.WindowErr = measureErr
	}
	if p := pet.Inspect(s); p.Summoned {
		r.PetFood = &PetFood{FoodTypes: p.Shown.FoodTypes, Checked: pet.CheckFood(s)}
		if !r.PetFood.Checked.Known {
			r.PetFood.InBags = pet.BagFood(s)
		}
	}
	return r
}

// LowStock lists, by name, the items that run out within an hour of play, ammo included.
func (r Report) LowStock() []string {
	var out []string
	for _, g := range r.Groups {
		for _, l := range g.Lines {
			if l.Low() {
				out = append(out, l.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Shortage is a consumable whose stock runs out within an hour of play (at the measured rate).
type Shortage struct {
	Name, Category string
	Total          int64
	PerHour        float64
	HoursLeft      float64
}

// Shortages are the items with too little stock except ammo, which has its own tool (wow_ammo). The rate and
// hours are rounded as the answer shows them, and the shopping list is computed from those figures.
func (r Report) Shortages() []Shortage {
	var out []Shortage
	for _, g := range r.Groups {
		if g.IsAmmo() {
			continue
		}
		for _, l := range g.Lines {
			if !l.HasHoursLeft || snapshot.Round1(l.HoursLeft) >= lowStockHours {
				continue
			}
			out = append(out, Shortage{Name: l.Name, Category: g.key, Total: l.Total(),
				PerHour: snapshot.Round1(l.Use.PerHour), HoursLeft: snapshot.Round1(l.HoursLeft)})
		}
	}
	return out
}
