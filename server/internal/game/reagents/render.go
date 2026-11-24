// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reagents

import (
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// howConsumed explains what the measured consumption means.
const howConsumed = "sum of decreases in the number of pieces in bags and on the character between consecutive dumps (purchases and increases do not count); this is an approximation, because a loss may also be a sale, mailing, banking or destruction"

// View describes the stocks of consumable items and their consumption from history.
func View(name string, s snapshot.Snapshot, tr progress.Tracker) map[string]any {
	return Inspect(name, s, tr).Render()
}

func (r Report) Render() map[string]any {
	if r.Empty() {
		return map[string]any{"note": "There is no ammo, food, bandages, potions or reagents in the bags (types Projectile, Consumable and Reagent). Open the bags in game and run /mcp reload."}
	}
	categories := []any{}
	for _, g := range r.Groups {
		lines := make([]any, len(g.Lines))
		for i, l := range g.Lines {
			lines[i] = l.render()
		}
		categories = append(categories, map[string]any{"category": g.key, "label": g.label, "items": lines})
	}
	out := map[string]any{"categories": categories}
	if r.HasWindow {
		u := r.Window
		out["consumption"] = map[string]any{"window": u.Window, "basis": u.Basis, "snapshots": u.Snapshots,
			"playHours": snapshot.Round1(u.Hours), "how": howConsumed}
	} else {
		out["consumption"] = map[string]any{"note": r.WindowErr}
	}
	if low := r.LowStock(); len(low) > 0 {
		out["lowStock"] = low
		out["lowStockNote"] = "lasts less than an hour of play at the measured rate"
	}
	if r.PetFood != nil {
		out["petFood"] = r.PetFood.render()
	}
	out["note"] = "Stock (total) is the bags plus what you wear (ammo); inBank is the state from the last bank visit and is not included in hoursLeft."
	return out
}

func (l Line) render() map[string]any {
	e := map[string]any{"name": l.Name, "inBags": l.Holdings.Bags, "total": l.Total()}
	if l.Holdings.Equipped > 0 {
		e["equipped"] = l.Holdings.Equipped
	}
	if l.Holdings.Bank > 0 {
		e["inBank"] = l.Holdings.Bank
	}
	if l.Measured {
		e["usedInWindow"] = l.Use.Used
		e["perHour"] = snapshot.Round1(l.Use.PerHour)
		if l.HasHoursLeft {
			e["hoursLeft"] = snapshot.Round1(l.HoursLeft)
			e["timeLeft"] = snapshot.FormatSeconds(int64(l.HoursLeft * 3600))
		}
	}
	return e
}

// render is the client's verdict on the bag food when the dump has it, otherwise food by item subtype with FoodNote.
func (f *PetFood) render() map[string]any {
	if f.Checked.Known {
		out := f.Checked.Render()
		out["foodTypes"] = f.FoodTypes
		return out
	}
	return map[string]any{"foodTypes": f.FoodTypes, "foodInBags": pet.RenderFood(f.InBags), "note": pet.FoodNote}
}
