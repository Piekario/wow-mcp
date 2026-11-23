// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"math"

	"wowmcp/internal/game/ammo"
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/reagents"
	"wowmcp/internal/snapshot"
)

// maxSellers is how many vendors of an item we show.
const maxSellers = 3

// Purchase is buying n pieces from the cheapest remembered vendor.
type Purchase struct {
	Npc          string
	Packs, Stack int64
	Cost         int64
}

// planPurchase is nil when n is zero or no remembered vendor sells the item.
func planPurchase(s snapshot.Snapshot, name string, n int64) *Purchase {
	sellers := items.SellersOf(s, name)
	if n <= 0 || len(sellers) == 0 {
		return nil
	}
	best := sellers[0]
	packs, cost := best.CostFor(n)
	return &Purchase{Npc: best.Npc, Packs: packs, Stack: best.Stack, Cost: cost}
}

func costOf(p *Purchase) int64 {
	if p == nil {
		return 0
	}
	return p.Cost
}

// cheapestSellers are the vendors of an item, cheapest first; empty when no remembered vendor has it.
func cheapestSellers(s snapshot.Snapshot, name string) []items.Seller {
	sellers := items.SellersOf(s, name)
	return sellers[:min(len(sellers), maxSellers)]
}

// RepairPlan is the gear's durability and the repair quote of the last vendor that repairs.
type RepairPlan struct {
	Present bool
	Wear    character.DurabilityReport
	Needed  bool
	Cost    int64
	// QuoteNpc and QuoteCaptured describe the quote; they are set only when the gear needs repair.
	QuoteNpc      any
	QuoteCaptured snapshot.Captured
	Menders       []string
}

func planRepair(s snapshot.Snapshot) RepairPlan {
	wear, ok := character.InspectDurability(s)
	if !ok {
		return RepairPlan{}
	}
	p := RepairPlan{Present: true, Wear: wear, Needed: wear.Percent < 100, Menders: readMenders(s)}
	if wear.HasRepair && p.Needed {
		p.Cost, p.QuoteNpc, p.QuoteCaptured = wear.Repair.CostCopper, wear.Repair.NPC, wear.Repair.Captured
	}
	return p
}

// AmmoPlan is how much ammo to buy for the planned hours of play. Known: the usage rate is known, so Needed and ToBuy mean something.
type AmmoPlan struct {
	Forecast ammo.Forecast
	Known    bool
	Needed   int64
	ToBuy    int64
	Buy      *Purchase
	Cost     int64
	Sellers  []items.Seller
}

// planAmmo is nil when the character has no ammo.
func planAmmo(name string, s snapshot.Snapshot, tr progress.Tracker, hours float64) *AmmoPlan {
	f := ammo.Left(name, s, tr)
	if !f.Present {
		return nil
	}
	p := &AmmoPlan{Forecast: f}
	if f.RateKnown {
		p.Known = true
		p.Needed = int64(math.Ceil(f.PerHour * hours))
		p.ToBuy = max(p.Needed-f.Stock.Total(), 0)
		p.Buy = planPurchase(s, f.Stock.Name, p.ToBuy)
		p.Cost = costOf(p.Buy)
	}
	p.Sellers = cheapestSellers(s, f.Stock.Name)
	return p
}

// Unpriced names the ammo when it must be bought but no remembered vendor has it.
func (p *AmmoPlan) Unpriced() []string {
	if p.Known && p.ToBuy > 0 && p.Buy == nil {
		return []string{p.Forecast.Stock.Name}
	}
	return nil
}

// FoodOffer is a food a remembered vendor sells; Item is shown as the dump has it.
type FoodOffer struct {
	Npc   string
	Item  any
	Price int64
	Stack int64
}

// PetFoodPlan is the pet food stock and what vendors offer.
type PetFoodPlan struct {
	FoodTypes any
	InBags    int64
	Offers    []FoodOffer
}

// planPetFood is nil when the character has no summoned pet.
func planPetFood(s snapshot.Snapshot) *PetFoodPlan {
	p := pet.Inspect(s)
	if !p.Summoned {
		return nil
	}
	return &PetFoodPlan{FoodTypes: p.Shown.FoodTypes, InBags: pet.BagFoodCount(s), Offers: readVendorFood(s)}
}

// ConsumableLine is an item about to run out and what buying it for the planned hours takes.
type ConsumableLine struct {
	reagents.Shortage
	ToBuy int64
	Buy   *Purchase
}

// ConsumablePlan points out items (bandages, potions, reagents, food) that are about to run out at the rate from history.
type ConsumablePlan struct {
	Lines   []ConsumableLine
	Cost    int64
	Missing []string // to buy, but no remembered vendor has them
}

// planConsumables is nil when nothing is running out.
func planConsumables(name string, s snapshot.Snapshot, tr progress.Tracker, hours float64) *ConsumablePlan {
	low := reagents.Inspect(name, s, tr).Shortages()
	if len(low) == 0 {
		return nil
	}
	p := &ConsumablePlan{}
	for _, sh := range low {
		needed := int64(math.Ceil(sh.PerHour * hours))
		line := ConsumableLine{Shortage: sh, ToBuy: max(needed-sh.Total, 0)}
		line.Buy = planPurchase(s, sh.Name, line.ToBuy)
		switch {
		case line.Buy != nil:
			p.Cost += line.Buy.Cost
		case line.ToBuy > 0:
			p.Missing = append(p.Missing, sh.Name)
		}
		p.Lines = append(p.Lines, line)
	}
	return p
}
