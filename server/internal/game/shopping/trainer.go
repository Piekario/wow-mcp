// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"sort"
	"strings"

	"wowmcp/internal/game/trainer"
	"wowmcp/internal/snapshot"
)

type skill struct {
	name, rank    string
	cost          int64
	costKnown     bool
	levelRequired int64
	npcs          []string // who it can be bought from (cheapest first)
}

func (sk skill) label() string {
	if sk.rank != "" {
		return sk.name + " " + sk.rank
	}
	return sk.name
}

// trainerPlan is the purchases from trainers: the offers of each remembered NPC and the total after removing duplicates.
type trainerPlan struct {
	offers    []any
	skills    []skill
	total     int64
	money     int64
	remembers int
}

// planTrainer gathers the available services (category = available) from all remembered trainers.
// The same skill at several trainers counts once, at the lowest price.
func planTrainer(s snapshot.Snapshot, level, money int64, fresh snapshot.Freshness) trainerPlan {
	plan := trainerPlan{money: money}
	byKey := map[string]*skill{}
	for _, v := range trainer.GoldOffers(s) {
		plan.remembers++
		plan.offers = append(plan.offers, offerView(v, level, money, fresh))
		for _, raw := range trainer.ServicesOf(v.Section, "available") {
			svc := snapshot.AsMap(raw)
			name, _ := svc["name"].(string)
			rank, _ := svc["rank"].(string)
			key := strings.ToLower(name + "|" + rank)
			cost, known := snapshot.AsInt(svc["costCopper"])
			sk := byKey[key]
			if sk == nil {
				req, _ := snapshot.AsInt(svc["levelRequired"])
				sk = &skill{name: name, rank: rank, cost: cost, costKnown: known, levelRequired: req}
				byKey[key] = sk
			} else if known && (!sk.costKnown || cost < sk.cost) {
				sk.cost, sk.costKnown = cost, true
			}
			sk.npcs = append(sk.npcs, v.Npc)
		}
	}
	for _, sk := range byKey {
		plan.skills = append(plan.skills, *sk)
	}
	sort.Slice(plan.skills, func(i, j int) bool {
		a, b := plan.skills[i], plan.skills[j]
		if a.levelRequired != b.levelRequired {
			return a.levelRequired < b.levelRequired
		}
		return a.label() < b.label()
	})
	for _, sk := range plan.skills {
		if sk.costKnown {
			plan.total += sk.cost
		}
	}
	return plan
}

// offerView describes one trainer's offer: available services with price and offer age flags.
func offerView(v snapshot.Visit, level, money int64, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"npc": v.Npc}
	snapshot.Merge(out, fresh.CapturedMeta(v.Section))
	if at, ok := v.CapturedAt(); ok && fresh.IsStale(at) {
		out["stale"] = true
	}
	var total int64
	services := []any{}
	for _, raw := range trainer.ServicesOf(v.Section, "available") {
		svc := snapshot.AsMap(raw)
		e := map[string]any{"name": svc["name"]}
		if svc["rank"] != nil {
			e["rank"] = svc["rank"]
		}
		if req, ok := snapshot.AsInt(svc["levelRequired"]); ok {
			e["levelRequired"] = req
		}
		if cost, ok := snapshot.AsInt(svc["costCopper"]); ok {
			e["cost"] = snapshot.Money(cost).String()
			e["canAfford"] = cost <= money
			total += cost
		}
		services = append(services, e)
	}
	out["services"] = services
	out["totalCost"] = snapshot.Money(total).String()
	if names := becameAvailable(v.Section, level); len(names) > 0 {
		out["maybeOutdated"] = map[string]any{
			"services": names,
			"note": "These services were unavailable when the offer was saved, and their required level is lower than the character's current level, " +
				"so the offer may have been saved at a lower level (assumption: the reason was the level; it could have been another, e.g. a prerequisite earlier rank). Open the trainer window again.",
		}
	}
	return out
}

// becameAvailable returns services saved as unavailable whose required level the character already has.
func becameAvailable(offer map[string]any, level int64) []string {
	var out []string
	for _, raw := range trainer.ServicesOf(offer, "unavailable") {
		svc := snapshot.AsMap(raw)
		if req, ok := snapshot.AsInt(svc["levelRequired"]); ok && req > 0 && req <= level {
			name, _ := svc["name"].(string)
			if rank, _ := svc["rank"].(string); rank != "" {
				name += " " + rank
			}
			out = append(out, name)
		}
	}
	return out
}

func (p trainerPlan) view() map[string]any {
	if p.remembers == 0 {
		return map[string]any{"note": "No saved trainer offers. Open the class (and profession) trainer window in game, then type /mcp reload."}
	}
	skills := make([]any, len(p.skills))
	for i, sk := range p.skills {
		e := map[string]any{"skill": sk.label(), "levelRequired": sk.levelRequired, "npcs": sk.npcs}
		if sk.costKnown {
			e["cost"] = snapshot.Money(sk.cost).String()
			e["canAfford"] = sk.cost <= p.money
		}
		skills[i] = e
	}
	out := map[string]any{"offers": p.offers, "toLearn": skills, "totalCost": snapshot.Money(p.total).String(), "canAffordAll": p.total <= p.money}
	if len(p.skills) == 0 {
		out["note"] = "The remembered offers have no services to buy right now (available)."
	}
	if p.total > p.money {
		out["shortBy"] = snapshot.Money(p.total - p.money).String()
	}
	return out
}
