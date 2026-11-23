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

// Service is one thing a trainer teaches. The cost is copper, except in a Beast Training window (see IsPetTraining).
// raw is the dump's own entry: the answer shows it with the price added.
type Service struct {
	Category   string
	CostCopper int64
	HasCost    bool
	raw        rawService
}

// Offer is what one trainer window showed. A real dump (Forever 1.60.1) had an offer without an NPC name and with a
// single Arcane Resistance Rank 1 service for 5: it looks like a Beast Training window opened from a spell.
type Offer struct {
	NPC      string
	Captured snapshot.Captured
	Services []Service
	section  rawSection
}

// IsPetTraining says whether the window has no NPC name, i.e. (assumption, not confirmed in game) it is the pet's
// Beast Training opened from a spell: its cost is training points, not copper.
func (o Offer) IsPetTraining() bool { return o.NPC == "" }

func (sv Service) CanAfford(money int64) bool { return sv.CostCopper <= money }

// ServicesIn are the offer's services of one category (available, unavailable, used).
func (o Offer) ServicesIn(category string) []Service {
	var out []Service
	for _, sv := range o.Services {
		if sv.Category == category {
			out = append(out, sv)
		}
	}
	return out
}

// Report is the answer to "what does this trainer teach and can I pay for it". Offer is nil when there is none to
// show; UnknownNpc is the name asked for when no remembered trainer matches it.
type Report struct {
	Money      int64
	Offer      *Offer
	UnknownNpc string
	Known      []string
}

// Inspect picks the offer (the last one or the one named by npc) and lists the remembered trainers.
func Inspect(s snapshot.Snapshot, q Query) Report {
	offers := Offers(s)
	_, money := s.LevelAndMoney()
	r := Report{Money: money, Known: snapshot.VisitNames(offers)}
	section := s.Map("trainer")
	if q.Npc != "" {
		v, ok := snapshot.FindVisit(offers, q.Npc)
		if !ok {
			r.UnknownNpc = q.Npc
			return r
		}
		section = v.Section
	}
	r.Offer = readOffer(section)
	return r
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
