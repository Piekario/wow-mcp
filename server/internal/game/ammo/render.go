// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ammo

import (
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// consumptionHow explains how ammo usage is calculated.
const consumptionHow = "sum of drops in the count between consecutive dumps (purchases and increases do not count); this is an approximation, because the loss can have causes other than shooting"

// notShrunk is the note used when the ammo did not decrease within the history stretch.
const notShrunk = "The ammo did not decrease in this stretch of history, so there is nothing to compute a rate from."

// View describes the ammo stock, the per-hour usage from history and how much to buy for q.Hours hours.
func View(name string, s snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	r, err := Inspect(name, s, tr, q)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r Report) Render() map[string]any {
	if !r.Present {
		return map[string]any{"note": "The character has no ammo: the ammo slot is empty and there are no arrows or bullets in bags. " +
			"Ammo applies to characters with a ranged weapon that uses it (e.g. Hunter); if you have some, open your bags and run /mcp reload."}
	}
	out := map[string]any{"ammo": r.Main.render()}
	if len(r.Others) > 0 {
		views := make([]any, len(r.Others))
		for i, o := range r.Others {
			views[i] = o.render()
		}
		out["otherAmmoInBags"] = views
	}
	if r.SlotEmpty {
		out["equippedNote"] = "The ammo slot is empty; calculated from the most numerous ammo in bags."
	}
	if r.UseErr != "" {
		out["consumption"] = map[string]any{"note": r.UseErr}
		return out
	}
	out["consumption"] = r.Use.Describe(consumptionHow, notShrunk)
	if r.HasHoursLeft {
		out["hoursLeft"] = snapshot.Round1(r.HoursLeft)
		out["timeLeft"] = snapshot.FormatSeconds(int64(r.HoursLeft * 3600))
		out["plan"] = r.Plan.render()
	}
	return out
}

func (st Stock) render() map[string]any {
	return map[string]any{"name": st.Name, "equipped": st.Equipped, "inBags": st.InBags, "total": st.Total()}
}

func (p PurchasePlan) render() map[string]any {
	out := map[string]any{"hours": p.Hours, "needed": p.Needed, "have": p.Have, "toBuy": p.ToBuy}
	if p.ToBuy == 0 {
		out["note"] = "The stock will last for the planned play time."
	}
	if p.Vendor != nil {
		out["vendor"] = map[string]any{"npc": p.Vendor.Npc, "stack": p.Vendor.Stack, "note": "offer from the last vendor visit"}
	}
	return out
}
