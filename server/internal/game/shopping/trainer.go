// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// OfferedService is one thing a trainer teaches. Name and Rank are kept as the dump has them (they are shown unchanged).
type OfferedService struct {
	Category      string
	Name, Rank    any
	Cost          int64
	HasCost       bool
	LevelRequired int64
	HasLevel      bool
}

func (sv OfferedService) name() string { n, _ := sv.Name.(string); return n }
func (sv OfferedService) rank() string { r, _ := sv.Rank.(string); return r }

// Label is the skill's name with its rank: "Disengage Rank 1".
func (sv OfferedService) Label() string {
	if rank := sv.rank(); rank != "" {
		return sv.name() + " " + rank
	}
	return sv.name()
}

func (sv OfferedService) CanAfford(money int64) bool { return sv.Cost <= money }

// TrainerOffer is what one remembered trainer window showed.
type TrainerOffer struct {
	Npc           string
	Captured      snapshot.Captured
	CapturedAt    int64
	HasCapturedAt bool
	Services      []OfferedService
}

func (o TrainerOffer) in(category string) []OfferedService {
	var out []OfferedService
	for _, sv := range o.Services {
		if sv.Category == category {
			out = append(out, sv)
		}
	}
	return out
}

// OfferSummary is one trainer's available services with their total price and flags about the offer's age.
type OfferSummary struct {
	Npc       string
	Captured  snapshot.Captured
	Stale     bool
	Available []OfferedService
	Total     int64
	// MaybeOutdated are services saved as unavailable whose required level the character already has: the offer
	// may have been saved at a lower level.
	MaybeOutdated []OfferedService
}

// Skill is a service to buy; the same skill at several trainers counts once, at the lowest price.
type Skill struct {
	Name, Rank    string
	Cost          int64
	CostKnown     bool
	LevelRequired int64
	NPCs          []string // who it can be bought from, in the order the offers came
}

func (sk Skill) Label() string {
	if sk.Rank != "" {
		return sk.Name + " " + sk.Rank
	}
	return sk.Name
}

// TrainerPlan is the purchases from trainers: the offers of each remembered NPC and the total after removing duplicates.
type TrainerPlan struct {
	Offers []OfferSummary
	Skills []Skill
	Total  int64
	Money  int64
}

// planTrainer gathers the available services from all remembered trainers.
func planTrainer(offers []TrainerOffer, level, money int64, fresh snapshot.Freshness) TrainerPlan {
	plan := TrainerPlan{Money: money}
	byKey := map[string]*Skill{}
	for _, o := range offers {
		plan.Offers = append(plan.Offers, summarize(o, level, fresh))
		for _, sv := range o.in("available") {
			key := strings.ToLower(sv.name() + "|" + sv.rank())
			sk := byKey[key]
			if sk == nil {
				sk = &Skill{Name: sv.name(), Rank: sv.rank(), Cost: sv.Cost, CostKnown: sv.HasCost, LevelRequired: sv.LevelRequired}
				byKey[key] = sk
			} else if sv.HasCost && (!sk.CostKnown || sv.Cost < sk.Cost) {
				sk.Cost, sk.CostKnown = sv.Cost, true
			}
			sk.NPCs = append(sk.NPCs, o.Npc)
		}
	}
	for _, sk := range byKey {
		plan.Skills = append(plan.Skills, *sk)
	}
	sort.Slice(plan.Skills, func(i, j int) bool {
		a, b := plan.Skills[i], plan.Skills[j]
		if a.LevelRequired != b.LevelRequired {
			return a.LevelRequired < b.LevelRequired
		}
		return a.Label() < b.Label()
	})
	for _, sk := range plan.Skills {
		if sk.CostKnown {
			plan.Total += sk.Cost
		}
	}
	return plan
}

func summarize(o TrainerOffer, level int64, fresh snapshot.Freshness) OfferSummary {
	sum := OfferSummary{Npc: o.Npc, Captured: o.Captured, Available: o.in("available")}
	sum.Stale = o.HasCapturedAt && fresh.IsStale(o.CapturedAt)
	for _, sv := range sum.Available {
		if sv.HasCost {
			sum.Total += sv.Cost
		}
	}
	for _, sv := range o.in("unavailable") {
		if sv.HasLevel && sv.LevelRequired > 0 && sv.LevelRequired <= level {
			sum.MaybeOutdated = append(sum.MaybeOutdated, sv)
		}
	}
	return sum
}

func (p TrainerPlan) Remembered() bool { return len(p.Offers) > 0 }

func (p TrainerPlan) CanAffordAll() bool { return p.Total <= p.Money }
