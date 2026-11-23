// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"math"

	"wowmcp/internal/game/ammo"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

type repairPlan struct {
	present    bool
	durability map[string]any
	cost       int64
	quoteNpc   any
	quoteAge   map[string]any
	menders    []string
	needed     bool
}

// planRepair uses durability (character.Durability) and the quote from the last vendor that repairs.
func planRepair(s snapshot.Snapshot, fresh snapshot.Freshness) repairPlan {
	d := character.Durability(s, fresh)
	if d == nil {
		return repairPlan{}
	}
	p := repairPlan{present: true, durability: d}
	if pct, ok := snapshot.AsFloat(d["overallPercent"]); ok {
		p.needed = pct < 100
	}
	if v := s.Map("vendor"); v != nil {
		if cost, ok := snapshot.AsInt(v["repairCostCopper"]); ok && p.needed {
			p.cost, p.quoteNpc = cost, v["npc"]
			p.quoteAge = fresh.CapturedMeta(v)
		}
	}
	for _, v := range s.Visits("vendor", "vendors") {
		if v.Section["canRepair"] == true {
			p.menders = append(p.menders, v.Npc)
		}
	}
	return p
}

func (p repairPlan) view() map[string]any {
	out := map[string]any{"overallPercent": p.durability["overallPercent"], "lowest": p.durability["lowest"], "needed": p.needed}
	if v, ok := p.durability["nearlyBroken"]; ok {
		out["nearlyBroken"] = v
	}
	if p.cost > 0 {
		out["cost"] = snapshot.Money(p.cost).String()
		out["quoteFrom"] = p.quoteNpc
		snapshot.Merge(out, p.quoteAge)
		out["costNote"] = "quote from the last window of a vendor that repairs; the current cost depends on the current durability"
	} else if p.needed {
		out["costNote"] = "no repair quote (no vendor has been opened since the gear was damaged)"
	}
	if len(p.menders) > 0 {
		out["canRepairAt"] = p.menders
	}
	return out
}

type ammoPlan struct {
	f      ammo.Forecast
	hours  float64
	toBuy  int64
	known  bool // the rate is known, so toBuy makes sense
	buy    map[string]any
	cost   int64
	sell   []any
	needed int64
}

// planAmmo computes how much ammo to buy for hours of play; nil when the character has no ammo.
func planAmmo(name string, s snapshot.Snapshot, tr progress.Tracker, hours float64) *ammoPlan {
	f := ammo.Left(name, s, tr)
	if !f.Present {
		return nil
	}
	p := &ammoPlan{f: f, hours: hours}
	if f.RateKnown {
		p.known = true
		p.needed = int64(math.Ceil(f.PerHour * hours))
		p.toBuy = max(p.needed-f.Stock.Total(), 0)
		p.buy, p.cost = buyPlan(s, f.Stock.Name, p.toBuy)
	}
	p.sell = sellersView(s, f.Stock.Name)
	return p
}

func (p *ammoPlan) unknown() []string {
	if p.known && p.toBuy > 0 && p.buy == nil {
		return []string{p.f.Stock.Name}
	}
	return nil
}

func (p *ammoPlan) view() map[string]any {
	out := map[string]any{"name": p.f.Stock.Name, "total": p.f.Stock.Total()}
	switch {
	case !p.known:
		out["note"] = "no consumption data: " + p.f.Note
	case !p.f.Burning:
		out["perHour"] = snapshot.Round1(p.f.PerHour)
		out["toBuy"] = int64(0)
		out["note"] = "the ammo did not decrease in the measured stretch of history, so there is nothing to compute a purchase from"
	default:
		out["perHour"] = snapshot.Round1(p.f.PerHour)
		out["hoursLeft"] = snapshot.Round1(p.f.HoursLeft)
		out["needed"] = p.needed
		out["toBuy"] = p.toBuy
	}
	if p.buy != nil {
		out["buyAt"] = p.buy
	}
	if len(p.sell) > 0 {
		out["sellers"] = p.sell
	} else if p.known && p.toBuy > 0 {
		out["sellersNote"] = "no remembered vendor has this ammo; open an ammo vendor window and run /mcp reload"
	}
	return out
}

type petFoodPlan struct {
	foodTypes any
	inBags    int64
	offers    []any
}

// planPetFood describes the pet food stock; nil when the character has no summoned pet.
func planPetFood(s snapshot.Snapshot) *petFoodPlan {
	if !pet.Exists(s) {
		return nil
	}
	p := &petFoodPlan{foodTypes: s.Map("pet")["foodTypes"], inBags: petFoodCount(s)}
	for _, v := range s.Visits("vendor", "vendors") {
		for _, raw := range snapshot.AsList(v.Section["items"]) {
			item := snapshot.AsMap(raw)
			if !itemkind.IsFood(item) {
				continue
			}
			price, _ := snapshot.AsInt(item["priceCopper"])
			stack, _ := snapshot.AsInt(item["stack"])
			p.offers = append(p.offers, map[string]any{
				"npc": v.Npc, "item": item["name"], "pack": snapshot.Money(price).String(), "stack": max(stack, 1),
			})
		}
	}
	return p
}

func (p *petFoodPlan) view() map[string]any {
	out := map[string]any{"foodInBags": p.inBags}
	if p.foodTypes != nil {
		out["foodTypes"] = p.foodTypes
	}
	if p.inBags == 0 {
		out["note"] = "there is no food in the bags that can be given to the pet (subtype Food); a hungry pet loses happiness"
	}
	if len(p.offers) > 0 {
		out["vendorFood"] = p.offers
		out["vendorFoodNote"] = pet.FoodNote
	}
	return out
}

type consumablePlan struct {
	lines []map[string]any
	cost  int64
	miss  []string
}

// planConsumables points out items (bandages, potions, reagents, food) that are about to run out at the rate from history.
func planConsumables(name string, s snapshot.Snapshot, tr progress.Tracker, hours float64) *consumablePlan {
	low := reagentShortages(name, s, tr)
	if len(low) == 0 {
		return nil
	}
	p := &consumablePlan{}
	for _, sh := range low {
		needed := int64(math.Ceil(sh.PerHour * hours))
		toBuy := max(needed-sh.Total, 0)
		e := map[string]any{
			"name": sh.Name, "category": sh.Category, "total": sh.Total,
			"perHour": snapshot.Round1(sh.PerHour), "hoursLeft": snapshot.Round1(sh.HoursLeft), "toBuy": toBuy,
		}
		buy, cost := buyPlan(s, sh.Name, toBuy)
		switch {
		case buy != nil:
			e["buyAt"] = buy
			p.cost += cost
		case toBuy > 0:
			e["sellersNote"] = "no remembered vendor has this item (it may be a profession product or must be bought from another NPC)"
			p.miss = append(p.miss, sh.Name)
		}
		p.lines = append(p.lines, e)
	}
	return p
}

func (p *consumablePlan) unknown() []string { return p.miss }

func (p *consumablePlan) view() map[string]any {
	list := make([]any, len(p.lines))
	for i, l := range p.lines {
		list[i] = l
	}
	return map[string]any{"lowStock": list, "note": "lasts less than an hour of play at the measured rate; quantities are computed for the planned play time; the rate is an approximation from dump history"}
}
