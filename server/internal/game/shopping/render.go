// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"fmt"

	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// noData is the text for missing data in forecasts.
const noData = "no data"

// View builds the shopping list answer: trainer, repair, ammo, pet food, materials, budget and return to town.
func View(name string, s snapshot.Snapshot, tr progress.Tracker, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	l, err := Inspect(name, s, tr, q, fresh)
	if err != nil {
		return nil, err
	}
	return l.Render(fresh), nil
}

func (l List) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{
		"money": snapshot.Money(l.Money).String(), "level": l.Level, "planningHours": l.Hours,
		"order":   "purchase priority: trainer skills, repair, ammo, food and materials",
		"trainer": l.Trainer.render(fresh),
	}
	if l.Repair.Present {
		out["repair"] = l.Repair.render(fresh)
	}
	if l.Ammo != nil {
		out["ammo"] = l.Ammo.render()
	}
	if l.PetFood != nil {
		out["petFood"] = l.PetFood.render()
	}
	if l.Consumables != nil {
		out["consumables"] = l.Consumables.render()
	}
	out["budget"] = l.Budget.render()
	out["returnToTown"] = l.Return.Render()
	out["note"] = "Vendor prices come from remembered windows (vendors) and may be old; the price in the vendor window is per pack (stack), the price of one piece is the pack price divided by its size. " +
		"The server buys, teaches and repairs nothing."
	return out
}

func (b Budget) render() map[string]any {
	lines := []any{}
	for _, line := range b.Lines {
		e := map[string]any{"what": line.What, "cost": snapshot.Money(line.Cost).String(), "covered": line.Covered}
		if line.Covered {
			e["leftAfter"] = snapshot.Money(line.LeftAfter).String()
		} else {
			e["shortBy"] = snapshot.Money(line.ShortBy).String()
		}
		lines = append(lines, e)
	}
	out := map[string]any{"steps": lines, "totalCost": snapshot.Money(b.Total).String(), "canAffordAll": b.CanAffordAll}
	if b.Over {
		out["shortBy"] = snapshot.Money(b.ShortBy).String()
	} else {
		out["leftAfterAll"] = snapshot.Money(b.LeftAfterAll).String()
	}
	if len(b.Unpriced) > 0 {
		out["unpriced"] = b.Unpriced
		out["unpricedNote"] = "these items are needed, but no remembered vendor has them, so they are not included in the cost"
	}
	return out
}

func (p Purchase) render() map[string]any {
	return map[string]any{"npc": p.Npc, "packs": p.Packs, "stack": p.Stack, "cost": snapshot.Money(p.Cost).String()}
}

func (p TrainerPlan) render(fresh snapshot.Freshness) map[string]any {
	if !p.Remembered() {
		return map[string]any{"note": "No saved trainer offers. Open the class (and profession) trainer window in game, then type /mcp reload."}
	}
	offers := make([]any, len(p.Offers))
	for i, o := range p.Offers {
		offers[i] = o.render(p.Money, fresh)
	}
	skills := make([]any, len(p.Skills))
	for i, sk := range p.Skills {
		e := map[string]any{"skill": sk.Label(), "levelRequired": sk.LevelRequired, "npcs": sk.NPCs}
		if sk.CostKnown {
			e["cost"] = snapshot.Money(sk.Cost).String()
			e["canAfford"] = sk.Cost <= p.Money
		}
		skills[i] = e
	}
	out := map[string]any{"offers": offers, "toLearn": skills, "totalCost": snapshot.Money(p.Total).String(), "canAffordAll": p.CanAffordAll()}
	if len(p.Skills) == 0 {
		out["note"] = "The remembered offers have no services to buy right now (available)."
	}
	if !p.CanAffordAll() {
		out["shortBy"] = snapshot.Money(p.Total - p.Money).String()
	}
	return out
}

func (o OfferSummary) render(money int64, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"npc": o.Npc}
	snapshot.Merge(out, fresh.CapturedMetaOf(o.Captured))
	if o.Stale {
		out["stale"] = true
	}
	services := []any{}
	for _, sv := range o.Available {
		e := map[string]any{"name": sv.Name}
		if sv.Rank != nil {
			e["rank"] = sv.Rank
		}
		if sv.HasLevel {
			e["levelRequired"] = sv.LevelRequired
		}
		if sv.HasCost {
			e["cost"] = snapshot.Money(sv.Cost).String()
			e["canAfford"] = sv.CanAfford(money)
		}
		services = append(services, e)
	}
	out["services"] = services
	out["totalCost"] = snapshot.Money(o.Total).String()
	if len(o.MaybeOutdated) > 0 {
		names := make([]string, len(o.MaybeOutdated))
		for i, sv := range o.MaybeOutdated {
			names[i] = sv.Label()
		}
		out["maybeOutdated"] = map[string]any{
			"services": names,
			"note": "These services were unavailable when the offer was saved, and their required level is lower than the character's current level, " +
				"so the offer may have been saved at a lower level (assumption: the reason was the level; it could have been another, e.g. a prerequisite earlier rank). Open the trainer window again.",
		}
	}
	return out
}

func (p RepairPlan) render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"overallPercent": p.Wear.Percent, "lowest": p.Wear.Lowest.Label(), "needed": p.Needed}
	if len(p.Wear.NearlyBroken) > 0 {
		out["nearlyBroken"] = p.Wear.NearlyBrokenLabels()
	}
	if p.Cost > 0 {
		out["cost"] = snapshot.Money(p.Cost).String()
		out["quoteFrom"] = p.QuoteNpc
		snapshot.Merge(out, fresh.CapturedMetaOf(p.QuoteCaptured))
		out["costNote"] = "quote from the last window of a vendor that repairs; the current cost depends on the current durability"
	} else if p.Needed {
		out["costNote"] = "no repair quote (no vendor has been opened since the gear was damaged)"
	}
	if len(p.Menders) > 0 {
		out["canRepairAt"] = p.Menders
	}
	return out
}

func (p *AmmoPlan) render() map[string]any {
	f := p.Forecast
	out := map[string]any{"name": f.Stock.Name, "total": f.Stock.Total()}
	switch {
	case !p.Known:
		out["note"] = "no consumption data: " + f.Note
	case !f.Burning:
		out["perHour"] = snapshot.Round1(f.PerHour)
		out["toBuy"] = int64(0)
		out["note"] = "the ammo did not decrease in the measured stretch of history, so there is nothing to compute a purchase from"
	default:
		out["perHour"] = snapshot.Round1(f.PerHour)
		out["hoursLeft"] = snapshot.Round1(f.HoursLeft)
		out["needed"] = p.Needed
		out["toBuy"] = p.ToBuy
	}
	if p.Buy != nil {
		out["buyAt"] = p.Buy.render()
	}
	if len(p.Sellers) > 0 {
		sellers := make([]any, len(p.Sellers))
		for i, sl := range p.Sellers {
			sellers[i] = sl.Describe()
		}
		out["sellers"] = sellers
	} else if p.Known && p.ToBuy > 0 {
		out["sellersNote"] = "no remembered vendor has this ammo; open an ammo vendor window and run /mcp reload"
	}
	return out
}

func (p *PetFoodPlan) render() map[string]any {
	out := map[string]any{"foodInBags": p.InBags}
	if p.FoodTypes != nil {
		out["foodTypes"] = p.FoodTypes
	}
	switch {
	case p.Checked.Known:
		out["bagFood"] = p.Checked.Render()
	case p.InBags == 0:
		out["note"] = "there is no food in the bags that can be given to the pet (subtype Food); a hungry pet loses happiness"
	}
	if len(p.Offers) > 0 {
		offers := make([]any, len(p.Offers))
		for i, o := range p.Offers {
			offers[i] = map[string]any{"npc": o.Npc, "item": o.Item, "pack": snapshot.Money(o.Price).String(), "stack": o.Stack}
		}
		out["vendorFood"] = offers
		out["vendorFoodNote"] = pet.FoodNote
	}
	return out
}

func (p *ConsumablePlan) render() map[string]any {
	list := make([]any, len(p.Lines))
	for i, l := range p.Lines {
		e := map[string]any{
			"name": l.Name, "category": l.Category, "total": l.Total,
			"perHour": snapshot.Round1(l.PerHour), "hoursLeft": snapshot.Round1(l.HoursLeft), "toBuy": l.ToBuy,
		}
		switch {
		case l.Buy != nil:
			e["buyAt"] = l.Buy.render()
		case l.ToBuy > 0:
			e["sellersNote"] = "no remembered vendor has this item (it may be a profession product or must be bought from another NPC)"
		}
		list[i] = e
	}
	return map[string]any{"lowStock": list, "note": "lasts less than an hour of play at the measured rate; quantities are computed for the planned play time; the rate is an approximation from dump history"}
}

// ReturnToTown answers the question "when to return to town": in how many levels there are new trainer skills,
// in how many hours of play the bags fill up, for how many hours the ammo lasts and what the gear durability is.
// When there is too little history for a rate, the relevant field says "no data".
func ReturnToTown(name string, s snapshot.Snapshot, tr progress.Tracker, fresh snapshot.Freshness) map[string]any {
	return planDeparture(name, s, tr).Render()
}

func (d Departure) Render() map[string]any {
	out := map[string]any{"trainer": d.Trainer.render(), "bags": d.Bags.render()}
	if d.Ammo.Present {
		out["ammo"] = renderAmmoOutlook(d)
	}
	if d.Durability != nil {
		wear := map[string]any{"overallPercent": d.Durability.Percent, "lowest": d.Durability.Lowest.Label()}
		if d.GearNearlyBroken() {
			wear["nearlyBroken"] = d.Durability.NearlyBrokenLabels()
		}
		out["durability"] = wear
	}
	if now := d.returnNow(); len(now) > 0 {
		out["returnNow"] = now
	}
	if d.HasSoonest {
		out["soonestHours"] = snapshot.Round1(d.Soonest)
		out["soonestBecause"] = d.SoonestBecause
	} else {
		out["soonestHours"] = noData
	}
	out["note"] = "Forecasts rely on the rate from dump history (the same window as wow_progress), so they are approximations; " +
		"purchases and sales between dumps distort the bag-filling rate."
	return out
}

// returnNow lists the reasons to go to town right now.
func (d Departure) returnNow() []string {
	var now []string
	if n := len(d.Trainer.AvailableNow); n > 0 {
		now = append(now, fmt.Sprintf("skills to buy from the trainer (%d)", n))
	}
	if d.BagsAlmostFull() {
		now = append(now, fmt.Sprintf("bags almost full (free slots: %d)", d.Bags.Slots.Free))
	}
	if d.AmmoUnderHour() {
		now = append(now, "ammo lasts less than an hour")
	}
	if d.GearNearlyBroken() {
		now = append(now, "gear close to breaking (below 25%)")
	}
	return now
}

func (t TrainerOutlook) render() map[string]any {
	if t.NoOffers {
		return map[string]any{"note": noData + ": no saved trainer offers (open a trainer window in game and run /mcp reload)"}
	}
	out := map[string]any{}
	if len(t.AvailableNow) > 0 {
		out["availableNow"] = t.AvailableNow
	}
	if t.Next == nil {
		out["next"] = noData + ": the remembered offers have no services with a higher required level"
	} else {
		out["next"] = map[string]any{"level": t.Next.Level, "levelsAway": t.Next.LevelsAway, "skills": t.Next.Skills, "trainers": t.Next.Trainers}
	}
	return out
}

func (b BagForecast) render() map[string]any {
	out := map[string]any{"freeSlots": b.Slots.Free, "totalSlots": b.Slots.Total}
	if b.Measured {
		out["gainedSlotsPerHour"] = snapshot.Round1(b.Use.PerHour)
		out["window"] = b.Use.Window
		if b.HasHoursUntilFull {
			out["hoursUntilFull"] = snapshot.Round1(b.HoursUntilFull)
			out["timeUntilFull"] = snapshot.FormatSeconds(int64(b.HoursUntilFull * 3600))
		} else {
			out["hoursUntilFull"] = noData
			out["note"] = "used bag slots did not grow in the measured stretch of history"
		}
		return out
	}
	out["hoursUntilFull"] = noData
	out["note"] = b.Err
	return out
}

func renderAmmoOutlook(d Departure) map[string]any {
	f := d.Ammo
	a := map[string]any{"name": f.Stock.Name, "total": f.Stock.Total()}
	switch {
	case !f.RateKnown:
		a["hoursLeft"] = noData
		a["note"] = f.Note
	case !f.Burning:
		a["hoursLeft"] = noData
		a["note"] = "ammo did not decrease in the measured stretch of history"
	default:
		a["hoursLeft"] = snapshot.Round1(f.HoursLeft)
		a["timeLeft"] = snapshot.FormatSeconds(int64(f.HoursLeft * 3600))
	}
	return a
}
