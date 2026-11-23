// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package trainer

import "wowmcp/internal/snapshot"

const noNpcNote = "An offer without an NPC name: the window was opened without talking to a trainer. Assumption (not confirmed in game): " +
	"it is the pet's Beast Training window, and costCopper is then the pet's training points, not copper, so cost and canAfford may make no sense."

// View describes one trainer's offer (the last one or the one given by npc) with prices and canAfford
// and the list of remembered trainers (known). Note: prices are written into the services in the dump itself
// (shared maps), so later reads of the same offer in the same call see them.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	return Inspect(s, q).Render(fresh)
}

func (r Report) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	if r.UnknownNpc != "" {
		out["money"] = snapshot.Money(r.Money).String()
		out["note"] = "I do not remember the trainer " + r.UnknownNpc + ". Remembered ones are in the known field; open that trainer's window in game and type /mcp reload."
		out["known"] = r.Known
		return out
	}
	if r.Offer != nil {
		out["trainer"] = r.Offer.section
	}
	r.Offer.priceInto(out, r.Money, fresh)
	if len(r.Known) > 0 {
		out["known"] = r.Known
	}
	return out
}

// PriceOffer adds to the response the character's gold, the age of the last trainer's offer, and for each service
// the price and canAfford (the same maps as in the dump, see View).
func PriceOffer(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	_, money := s.LevelAndMoney()
	readOffer(s.Map("trainer")).priceInto(out, money, fresh)
}

func (o *Offer) priceInto(out map[string]any, money int64, fresh snapshot.Freshness) {
	out["money"] = snapshot.Money(money).String()
	if o == nil {
		out["note"] = "No saved offer. Open a trainer window in game, then type /mcp reload."
		return
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(o.Captured))
	if o.IsPetTraining() {
		out["npcNote"] = noNpcNote
	}
	for _, sv := range o.Services {
		if sv.HasCost {
			sv.raw["cost"] = snapshot.Money(sv.CostCopper).String()
			sv.raw["canAfford"] = sv.CanAfford(money)
		}
	}
}
