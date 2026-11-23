// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package trainer

import "wowmcp/internal/snapshot"

type (
	rawSection = map[string]any
	rawService = map[string]any
)

// Offers returns the remembered trainer offers: the newest for each NPC, from the most recent visit.
func Offers(s snapshot.Snapshot) []snapshot.Visit { return s.Visits("trainer", "trainers") }

// IsPetTraining says whether the offer is a window without an NPC name (see Offer.IsPetTraining).
func IsPetTraining(offer map[string]any) bool {
	o := readOffer(offer)
	return o != nil && o.IsPetTraining()
}

// Services returns the last trainer's services of the given category (available, unavailable, used);
// empty when the last window is Beast Training (cost in training points, not copper).
func Services(s snapshot.Snapshot, category string) []any {
	if last := s.Map("trainer"); !IsPetTraining(last) {
		return ServicesOf(last, category)
	}
	return nil
}

// ServicesOf returns the services of a trainer's offer in the given category, as the dump has them.
func ServicesOf(offer map[string]any, category string) []any {
	var out []any
	if o := readOffer(offer); o != nil {
		for _, sv := range o.ServicesIn(category) {
			out = append(out, sv.raw)
		}
	}
	return out
}

// readOffer is nil for a missing section.
func readOffer(section rawSection) *Offer {
	if section == nil {
		return nil
	}
	o := &Offer{Captured: snapshot.CapturedOf(section), section: section}
	o.NPC, _ = section["npc"].(string)
	for _, raw := range snapshot.AsList(section["services"]) {
		m := snapshot.AsMap(raw)
		if m == nil {
			continue
		}
		sv := Service{raw: m}
		sv.Category, _ = m["category"].(string)
		sv.CostCopper, sv.HasCost = m["costCopper"].(int64)
		o.Services = append(o.Services, sv)
	}
	return o
}
