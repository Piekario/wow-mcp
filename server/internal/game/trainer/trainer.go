// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package trainer covers the offers of remembered class, profession and weapon trainers (the last one plus up to 10 earlier ones).
package trainer

import "wowmcp/internal/snapshot"

// Query is the input of wow_trainer.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Npc       string `json:"npc,omitempty" jsonschema:"name (or fragment) of one of the remembered trainers; empty = the most recently visited. The list of remembered ones is in the known field"`
}

// noNpcNote describes an offer without an NPC name. A real dump (Forever 1.60.1) had such an offer
// with a single Arcane Resistance Rank 1 service for 5: it looks like a Beast Training window opened from a spell.
const noNpcNote = "An offer without an NPC name: the window was opened without talking to a trainer. Assumption (not confirmed in game): " +
	"it is the pet's Beast Training window, and costCopper is then the pet's training points, not copper, so cost and canAfford may make no sense."

// Offers returns the remembered trainer offers: the newest for each NPC, from the most recent visit.
func Offers(s snapshot.Snapshot) []snapshot.Visit { return s.Visits("trainer", "trainers") }

// IsPetTraining says whether the offer is a window without an NPC name, i.e. (assumption) the pet's Beast Training
// opened from a spell: its cost is training points, not copper.
func IsPetTraining(offer map[string]any) bool {
	if offer == nil {
		return false
	}
	name, _ := offer["npc"].(string)
	return name == ""
}

// GoldOffers is the trainer offers paid in copper (without the Beast Training window); for shopping lists and budgets.
func GoldOffers(s snapshot.Snapshot) []snapshot.Visit {
	var out []snapshot.Visit
	for _, v := range Offers(s) {
		if !IsPetTraining(v.Section) {
			out = append(out, v)
		}
	}
	return out
}

// View describes one trainer's offer (the last one or the one given by npc) with prices and canAfford
// and the list of remembered trainers (known). Note: prices are written into the services in the dump itself
// (shared maps), so later reads of the same offer in the same call see them.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	offers := Offers(s)
	out := map[string]any{}
	section := s.Map("trainer")
	if q.Npc != "" {
		v, ok := snapshot.FindVisit(offers, q.Npc)
		if !ok {
			_, money := s.LevelAndMoney()
			out["money"] = snapshot.Money(money).String()
			out["note"] = "I do not remember the trainer " + q.Npc + ". Remembered ones are in the known field; open that trainer's window in game and type /mcp reload."
			out["known"] = snapshot.VisitNames(offers)
			return out
		}
		section = v.Section
	}
	if section != nil {
		out["trainer"] = section
	}
	priceSection(s, section, out, fresh)
	if names := snapshot.VisitNames(offers); len(names) > 0 {
		out["known"] = names
	}
	return out
}

// PriceOffer adds to the response the character's gold, the age of the last trainer's offer, and for each service
// the price and canAfford (the same maps as in the dump, see View).
func PriceOffer(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	priceSection(s, s.Map("trainer"), out, fresh)
}

func priceSection(s snapshot.Snapshot, trainer map[string]any, out map[string]any, fresh snapshot.Freshness) {
	_, money := s.LevelAndMoney()
	out["money"] = snapshot.Money(money).String()
	if trainer == nil {
		out["note"] = "No saved offer. Open a trainer window in game, then type /mcp reload."
		return
	}
	snapshot.Merge(out, fresh.CapturedMeta(trainer))
	if IsPetTraining(trainer) {
		out["npcNote"] = noNpcNote
	}
	for _, raw := range snapshot.AsList(trainer["services"]) {
		if svc, ok := raw.(map[string]any); ok {
			if cost, ok := svc["costCopper"].(int64); ok {
				svc["cost"] = snapshot.Money(cost).String()
				svc["canAfford"] = cost <= money
			}
		}
	}
}

// Services returns the last trainer's services of the given category (available, unavailable, used);
// empty when the last window is Beast Training (cost in training points, not copper).
func Services(s snapshot.Snapshot, category string) []any {
	if last := s.Map("trainer"); !IsPetTraining(last) {
		return ServicesOf(last, category)
	}
	return nil
}

// ServicesOf returns the services of a trainer's offer in the given category.
func ServicesOf(offer map[string]any, category string) []any {
	var out []any
	for _, raw := range snapshot.AsList(offer["services"]) {
		if svc := snapshot.AsMap(raw); svc["category"] == category {
			out = append(out, svc)
		}
	}
	return out
}
