// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package shopping is the shopping list before leaving town (trainer, repair, ammo, pet food,
// consumable materials) and a forecast of when you will have to return to town. It stitches together existing contexts
// (trainer, ammo, pet, reagents, character, items); it buys and repairs nothing itself.
package shopping

import (
	"fmt"
	"math"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_shopping_list.
type Query struct {
	Character string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Hours     float64 `json:"hours,omitempty" jsonschema:"for how many hours of play to compute the ammo and materials purchase; default 2"`
}

// defaultHours is the default play time we compute purchases for.
const defaultHours = 2

// List is the shopping list: what to buy in priority order, what the gold covers and when to return to town.
type List struct {
	Money, Level int64
	Hours        float64
	Trainer      TrainerPlan
	Repair       RepairPlan
	Ammo         *AmmoPlan
	PetFood      *PetFoodPlan
	Consumables  *ConsumablePlan
	Budget       Budget
	Return       Departure
}

// Inspect builds the shopping list for hours of play (q.Hours, default 2): trainer, repair, ammo, pet food,
// materials, budget and return to town.
func Inspect(name string, s snapshot.Snapshot, tr progress.Tracker, q Query, fresh snapshot.Freshness) (List, error) {
	hours := q.Hours
	switch {
	case hours == 0:
		hours = defaultHours
	case hours < 0 || math.IsNaN(hours):
		return List{}, fmt.Errorf("hours must be positive (default %d), got %g", defaultHours, q.Hours)
	}
	level, money := s.LevelAndMoney()
	l := List{Money: money, Level: level, Hours: hours}
	l.Trainer = planTrainer(readOffers(s), level, money, fresh)
	steps := []Step{{Label: "trainer skills", Cost: l.Trainer.Total}}

	l.Repair = planRepair(s)
	if l.Repair.Present {
		steps = append(steps, Step{Label: "repair", Cost: l.Repair.Cost})
	}
	if l.Ammo = planAmmo(name, s, tr, hours); l.Ammo != nil {
		steps = append(steps, Step{Label: "ammo", Cost: l.Ammo.Cost, Unpriced: l.Ammo.Unpriced()})
	}
	l.PetFood = planPetFood(s)
	food := Step{Label: "food and materials"}
	if l.Consumables = planConsumables(name, s, tr, hours); l.Consumables != nil {
		food.Cost, food.Unpriced = l.Consumables.Cost, l.Consumables.Missing
	}
	l.Budget = planBudget(money, append(steps, food))
	l.Return = planDeparture(name, s, tr)
	return l, nil
}

// Step is one budget item in priority order; Unpriced are items without a known price (not counted in the cost).
type Step struct {
	Label    string
	Cost     int64
	Unpriced []string
}

type BudgetLine struct {
	What      string
	Cost      int64
	Covered   bool
	LeftAfter int64 // when covered
	ShortBy   int64 // when not
}

// Budget checks in priority order what the gold covers; steps without a cost are skipped.
type Budget struct {
	Lines        []BudgetLine
	Total        int64
	CanAffordAll bool
	Over         bool // Total is more than the money
	ShortBy      int64
	LeftAfterAll int64
	Unpriced     []string
}

func planBudget(money int64, steps []Step) Budget {
	remaining := money
	b := Budget{}
	covered := true
	for _, st := range steps {
		b.Unpriced = append(b.Unpriced, st.Unpriced...)
		if st.Cost <= 0 {
			continue
		}
		b.Total += st.Cost
		line := BudgetLine{What: st.Label, Cost: st.Cost, Covered: st.Cost <= remaining}
		if line.Covered {
			remaining -= st.Cost
			line.LeftAfter = remaining
		} else {
			line.ShortBy = st.Cost - remaining
			covered = false
		}
		b.Lines = append(b.Lines, line)
	}
	b.CanAffordAll = covered && b.Total <= money
	if b.Over = b.Total > money; b.Over {
		b.ShortBy = b.Total - money
	} else {
		b.LeftAfterAll = money - b.Total
	}
	return b
}
