// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package shopping is the shopping list before leaving town (trainer, repair, ammo, pet food,
// consumable materials) and a forecast of when you will have to return to town. It stitches together existing contexts
// (trainer, ammo, pet, reagents, character, items); it buys and repairs nothing itself.
package shopping

import (
	"fmt"
	"math"

	"wowmcp/internal/game/items"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/reagents"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_shopping_list.
type Query struct {
	Character string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Hours     float64 `json:"hours,omitempty" jsonschema:"for how many hours of play to compute the ammo and materials purchase; default 2"`
}

// defaultHours is the default play time we compute purchases for.
const defaultHours = 2

// maxSellers is how many vendors of an item we show.
const maxSellers = 3

// step is one budget item in priority order.
type step struct {
	key, label string
	cost       int64
	unknown    []string // items without a known price (not counted in the cost)
}

// View builds the shopping list: trainer, repair, ammo, pet food, materials, budget and return to town.
func View(name string, s snapshot.Snapshot, tr progress.Tracker, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	hours := q.Hours
	switch {
	case hours == 0:
		hours = defaultHours
	case hours < 0 || math.IsNaN(hours):
		return nil, fmt.Errorf("hours must be positive (default %d), got %g", defaultHours, q.Hours)
	}
	level, money := s.LevelAndMoney()
	out := map[string]any{
		"money": snapshot.Money(money).String(), "level": level, "planningHours": hours,
		"order": "purchase priority: trainer skills, repair, ammo, food and materials",
	}
	var steps []step

	plan := planTrainer(s, level, money, fresh)
	out["trainer"] = plan.view()
	steps = append(steps, step{key: "trainer", label: "trainer skills", cost: plan.total})

	repair := planRepair(s, fresh)
	if repair.present {
		out["repair"] = repair.view()
		steps = append(steps, step{key: "repair", label: "repair", cost: repair.cost})
	}

	if a := planAmmo(name, s, tr, hours); a != nil {
		out["ammo"] = a.view()
		steps = append(steps, step{key: "ammo", label: "ammo", cost: a.cost, unknown: a.unknown()})
	}

	food := step{key: "food", label: "food and materials"}
	if p := planPetFood(s); p != nil {
		out["petFood"] = p.view()
	}
	if c := planConsumables(name, s, tr, hours); c != nil {
		out["consumables"] = c.view()
		food.cost, food.unknown = c.cost, c.unknown()
	}
	steps = append(steps, food)

	out["budget"] = budget(money, steps)
	out["returnToTown"] = ReturnToTown(name, s, tr, fresh)
	out["note"] = "Vendor prices come from remembered windows (vendors) and may be old; the price in the vendor window is per pack (stack), the price of one piece is the pack price divided by its size. " +
		"The server buys, teaches and repairs nothing."
	return out, nil
}

// budget checks in priority order what the gold covers; it skips items without a cost.
func budget(money int64, steps []step) map[string]any {
	remaining := money
	var total int64
	covered := true
	list := []any{}
	var unknown []string
	for _, st := range steps {
		unknown = append(unknown, st.unknown...)
		if st.cost <= 0 {
			continue
		}
		total += st.cost
		ok := st.cost <= remaining
		e := map[string]any{"what": st.label, "cost": snapshot.Money(st.cost).String(), "covered": ok}
		if ok {
			remaining -= st.cost
			e["leftAfter"] = snapshot.Money(remaining).String()
		} else {
			e["shortBy"] = snapshot.Money(st.cost - remaining).String()
			covered = false
		}
		list = append(list, e)
	}
	out := map[string]any{
		"steps": list, "totalCost": snapshot.Money(total).String(), "canAffordAll": covered && total <= money,
	}
	if total > money {
		out["shortBy"] = snapshot.Money(total - money).String()
	} else {
		out["leftAfterAll"] = snapshot.Money(money - total).String()
	}
	if len(unknown) > 0 {
		out["unpriced"] = unknown
		out["unpricedNote"] = "these items are needed, but no remembered vendor has them, so they are not included in the cost"
	}
	return out
}

// sellersView describes the cheapest vendors of an item; nil when no remembered vendor has it.
func sellersView(s snapshot.Snapshot, name string) []any {
	var out []any
	for i, sl := range items.SellersOf(s, name) {
		if i == maxSellers {
			break
		}
		out = append(out, sl.Describe())
	}
	return out
}

// buyPlan computes buying n pieces from the cheapest remembered vendor: description and cost (0 when nobody sells).
func buyPlan(s snapshot.Snapshot, name string, n int64) (view map[string]any, cost int64) {
	sellers := items.SellersOf(s, name)
	if n <= 0 || len(sellers) == 0 {
		return nil, 0
	}
	best := sellers[0]
	packs, cost := best.CostFor(n)
	return map[string]any{"npc": best.Npc, "packs": packs, "stack": best.Stack, "cost": snapshot.Money(cost).String()}, cost
}

// petFoodCount sums the food in bags that can be given to the pet.
func petFoodCount(s snapshot.Snapshot) int64 {
	var n int64
	for _, raw := range pet.FoodInBags(s) {
		c, _ := snapshot.AsInt(snapshot.AsMap(raw)["count"])
		n += c
	}
	return n
}

// reagentShortages are items with too little stock at the consumption rate from history.
func reagentShortages(name string, s snapshot.Snapshot, tr progress.Tracker) []reagents.Shortage {
	return reagents.LowStock(reagents.View(name, s, tr))
}
