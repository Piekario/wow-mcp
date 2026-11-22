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
func classify(item map[string]any) (category, bool) {
	switch {
	case itemkind.IsAmmo(item):
		return ammo, true
	case itemkind.Is(item, itemkind.ClassReagent):
		return reagent, true
	}
	switch itemkind.ConsumableKind(item) {
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

func snapshotName(item map[string]any) string {
	n, _ := item["name"].(string)
	return n
}

// stock is one consumable item of the character.
type stock struct {
	name string
	cat  category
}

// collect gathers consumable items from bags and equipped ammo, in a stable name order.
func collect(s snapshot.Snapshot) []stock {
	seen := map[string]category{}
	add := func(raw any) {
		item := snapshot.AsMap(raw)
		if cat, ok := classify(item); ok && snapshotName(item) != "" {
			seen[snapshotName(item)] = cat
		}
	}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			add(raw)
		}
	}
	for _, raw := range s.Equipment() {
		if snapshot.AsMap(raw)["slot"] == "ammo" {
			add(raw)
		}
	}
	list := make([]stock, 0, len(seen))
	for name, cat := range seen {
		list = append(list, stock{name, cat})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}

// howConsumed explains what the measured consumption means.
const howConsumed = "sum of decreases in the number of pieces in bags and on the character between consecutive dumps (purchases and increases do not count); this is an approximation, because a loss may also be a sale, mailing, banking or destruction"

// lowStockHours: with this many hours of play of stock the item goes into lowStock.
const lowStockHours = 1.0

// View describes the stocks of consumable items and their consumption from history.
func View(name string, s snapshot.Snapshot, tr progress.Tracker) map[string]any {
	stocks := collect(s)
	if len(stocks) == 0 {
		return map[string]any{"note": "There is no ammo, food, bandages, potions or reagents in the bags (types Projectile, Consumable and Reagent). Open the bags in game and run /mcp reload."}
	}
	windows, werr := consumption.Load(tr, name, s)

	byCat := map[string][]any{}
	var lowStock []string
	var window map[string]any
	var measureErr string
	for _, st := range stocks {
		h := items.Count(s, st.name)
		e := map[string]any{"name": st.name, "inBags": h.Bags, "total": h.Bags + h.Equipped}
		if h.Equipped > 0 {
			e["equipped"] = h.Equipped
		}
		if h.Bank > 0 {
			e["inBank"] = h.Bank
		}
		if werr == nil {
			use, err := windows.Measure(func(snap snapshot.Snapshot) int64 {
				c := items.Count(snap, st.name)
				return c.Bags + c.Equipped
			})
			if err != nil {
				measureErr = err.Error()
			} else {
				if window == nil {
					window = use.Describe(howConsumed, "")
					delete(window, "used")
					delete(window, "perHour")
					delete(window, "note")
				}
				e["usedInWindow"] = use.Used
				e["perHour"] = snapshot.Round1(use.PerHour)
				if left, ok := use.HoursLeft(h.Bags + h.Equipped); ok {
					e["hoursLeft"] = snapshot.Round1(left)
					e["timeLeft"] = snapshot.FormatSeconds(int64(left * 3600))
					if left < lowStockHours {
						lowStock = append(lowStock, st.name)
					}
				}
			}
		}
		byCat[st.cat.key] = append(byCat[st.cat.key], e)
	}

	categories := []any{}
	for _, cat := range orderedBy {
		if list := byCat[cat.key]; len(list) > 0 {
			categories = append(categories, map[string]any{"category": cat.key, "label": cat.label, "items": list})
		}
	}
	out := map[string]any{"categories": categories}
	switch {
	case werr != nil:
		out["consumption"] = map[string]any{"note": werr.Error()}
	case window == nil:
		out["consumption"] = map[string]any{"note": measureErr}
	default:
		out["consumption"] = window
	}
	if len(lowStock) > 0 {
		out["lowStock"] = lowStock
		out["lowStockNote"] = "lasts less than an hour of play at the measured rate"
	}
	if pet.Exists(s) {
		out["petFood"] = map[string]any{
			"foodTypes":  s.Map("pet")["foodTypes"],
			"foodInBags": pet.FoodInBags(s),
			"note":       pet.FoodNote,
		}
	}
	out["note"] = "Stock (total) is the bags plus what you wear (ammo); inBank is the state from the last bank visit and is not included in hoursLeft."
	return out
}

// Shortage is a consumable whose stock runs out within an hour of play (at the measured rate).
type Shortage struct {
	Name, Category string
	Total          int64
	PerHour        float64
	HoursLeft      float64
}

// LowStock picks from the View response the items with too little stock (hoursLeft under an hour) except ammo,
// which has its own tool (wow_ammo). It reads the response structure of the same package.
func LowStock(view map[string]any) []Shortage {
	var out []Shortage
	for _, raw := range snapshot.AsList(view["categories"]) {
		cat := snapshot.AsMap(raw)
		key, _ := cat["category"].(string)
		if key == ammo.key {
			continue
		}
		for _, rawItem := range snapshot.AsList(cat["items"]) {
			e := snapshot.AsMap(rawItem)
			left, ok := snapshot.AsFloat(e["hoursLeft"])
			if !ok || left >= lowStockHours {
				continue
			}
			total, _ := snapshot.AsInt(e["total"])
			perHour, _ := snapshot.AsFloat(e["perHour"])
			name, _ := e["name"].(string)
			out = append(out, Shortage{Name: name, Category: key, Total: total, PerHour: perHour, HoursLeft: left})
		}
	}
	return out
}
