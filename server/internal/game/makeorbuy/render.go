// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package makeorbuy

import (
	"fmt"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/snapshot"
)

// View compares the crafting cost with the finished item's price for the character's recipes.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	return Analyse(s, q, fresh).Render()
}

func (r Report) Render() map[string]any {
	if !r.Saved {
		return map[string]any{"note": "No saved recipes. Open the profession window in game (e.g. Leatherworking), then type /mcp reload. Each profession is saved separately."}
	}
	views := make([]any, len(r.Evaluations))
	for i, e := range r.Evaluations {
		views[i] = e.render()
	}
	out := map[string]any{
		"recipes": views, "shown": len(views), "total": r.Total, "byVerdict": r.ByVerdict,
		"assumptions": "A recipe yields 1 unit of the item with the same name (the server does not know the number of units per craft). A reagent is priced at the lowest buyout price per unit from the auction house, or at the remembered vendor's price if lower; it does not account for what the character already has in bags. margin = finished item price minus reagent cost; marginAfterCut subtracts " + fmt.Sprintf("%.0f%%", auctions.AuctionCut*100) + " auction cut when you want to sell it. A missing price is never counted as zero: then verdict=unknown and missingPrices. Prices older than 48 h are in stalePrices.",
	}
	if r.Truncated() {
		out["truncated"] = true
	}
	if r.NoAuctionPrices {
		out["hint"] = []string{"No auction house prices. Open the auction house, search for reagents and finished items, then /mcp reload."}
	}
	if r.HasVisit {
		out["auctionLastVisit"] = snapshot.FormatAge(r.LastVisitAge) + " ago"
	}
	return out
}

func (e Evaluation) render() map[string]any {
	lines := make([]any, len(e.Lines))
	for i, l := range e.Lines {
		lines[i] = l.render()
	}
	v := map[string]any{"name": e.Name, "profession": e.Profession, "reagents": lines}
	if e.ItemID > 0 {
		v["itemId"] = e.ItemID
	}
	switch {
	case e.Complete():
		v["reagentCost"], v["reagentCostCopper"] = snapshot.Money(e.Cost).String(), e.Cost
	case e.Cost > 0:
		v["knownPartOfCost"] = snapshot.Money(e.Cost).String() + " (incomplete: prices of some reagents are missing)"
	}
	if p := e.Product; p != nil {
		v["productPrice"], v["productPriceCopper"] = snapshot.Money(p.Copper).String(), p.Copper
		if p.HasAge {
			v["productSeenAgo"] = snapshot.FormatAge(p.Age)
		}
	}
	if e.Known() {
		v["margin"], v["marginCopper"] = snapshot.Money(e.Margin).Signed(), e.Margin
		v["marginAfterCut"] = snapshot.Money(e.MarginAfterCut()).Signed()
	}
	v["verdict"] = e.Verdict()
	if len(e.Missing) > 0 {
		v["missingPrices"] = missingLabels(e.Missing)
	}
	if len(e.Stale) > 0 {
		v["stalePrices"] = staleLabels(e.Stale)
	}
	return v
}

func (l ReagentLine) render() map[string]any {
	line := map[string]any{"name": l.Name, "need": l.Need}
	if l.Price == nil {
		line["unitPrice"] = nil
		return line
	}
	line["unitPrice"], line["unitPriceCopper"], line["source"] = snapshot.Money(l.Price.Copper).String(), l.Price.Copper, l.Price.Source
	line["lineTotal"] = snapshot.Money(l.Total()).String()
	if l.Price.HasAge {
		line["seenAgo"] = snapshot.FormatAge(l.Price.Age)
	}
	return line
}

func missingLabels(missing []MissingPrice) []string {
	labels := make([]string, len(missing))
	for i, m := range missing {
		labels[i] = m.Name
		if m.Finished {
			labels[i] += " (finished item)"
		}
	}
	return labels
}

func staleLabels(stale []StalePrice) []string {
	labels := make([]string, len(stale))
	for i, st := range stale {
		age := snapshot.FormatAge(st.Age)
		if st.Finished {
			labels[i] = fmt.Sprintf("%s (finished item, %s)", st.Name, age)
		} else {
			labels[i] = fmt.Sprintf("%s (%s)", st.Name, age)
		}
	}
	return labels
}
