// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package bagplan

import "wowmcp/internal/snapshot"

// View assesses every item in the bags and summarises how many slots and how much gold this yields.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Analyse(s, fresh).Render()
}

func (p Plan) Render() map[string]any {
	if len(p.Suggestions) == 0 {
		return map[string]any{
			"note":     "The bags are empty or the dump has no bags section.",
			"bagSlots": map[string]any{"totalSlots": p.Space.Total, "freeSlots": p.Space.Free},
		}
	}
	entries := make([]any, len(p.Suggestions))
	for i, sg := range p.Suggestions {
		entries[i] = sg.render()
	}
	out := map[string]any{
		"items": entries,
		"bagSlots": map[string]any{
			"total": p.Space.Total, "free": p.Space.Free, "freeAfterPlan": p.FreeAfter(),
		},
		"slotsFreed": p.SlotsFreed(),
		"summary":    p.renderSummary(),
		"howDecided": "Rule order: quest items, ammo for the equipped weapon, materials for known recipes and profession tools, consumables, auction offer (price after the cut with a 20% margin over the vendor), possible upgrades, then selling grey and ordinary items, while trade materials and better quality go to the bank.",
		"limits":     p.limitsText(),
	}
	if !p.Bank.Saved {
		out["bankNote"] = "No saved bank (it was never opened), so it is unknown whether there is room in it for items with the bank action."
	}
	return out
}

// limitsText says what the plan cannot know; binding is known only from addon 0.9.0 dumps (isBound, bindType).
func (p Plan) limitsText() string {
	binding := "The dump does not report binding (Soulbound) of bag items, so auctions are only a suggestion."
	if p.BindingKnown {
		binding = "Bound items are known from the dump (isBound, bindType) and are never suggested for the auction house."
	}
	return "The server cannot see what your other characters need, nor does it know every recipe (only those from opened profession windows, wow_recipes). " +
		binding + " The plan has no destroy or discard action; nothing is sold or removed, you make the decisions in game."
}

func (sg Suggestion) render() map[string]any {
	e := map[string]any{"name": sg.Name, "count": sg.Count, "slots": sg.Slots, "action": sg.Action, "reason": sg.Reason}
	if sg.Vendor > 0 {
		e["vendorValue"] = snapshot.Money(sg.Vendor).String()
	}
	if sg.Action == Auction {
		e["auctionAfterCut"] = snapshot.Money(sg.AuctionNet).String()
	}
	return e
}

// renderSummary gives totals for each action; the bank gets the free slots from the last visit.
func (p Plan) renderSummary() map[string]any {
	out := map[string]any{}
	for _, action := range actionOrder {
		t := p.Totals[action]
		if t == nil {
			continue
		}
		e := map[string]any{"items": t.Items, "slots": t.Slots}
		switch action {
		case Sell:
			e["vendorValue"] = snapshot.Money(t.Vendor).String()
		case Auction:
			e["afterCut"] = snapshot.Money(t.Auction).String()
			e["vendorInstead"] = snapshot.Money(t.Vendor).String()
		case Bank:
			if p.Bank.Saved {
				e["bankFreeSlots"] = p.Bank.FreeSlots
				e["fits"] = t.fitsBank(p.Bank)
			}
		}
		out[action] = e
	}
	return out
}
