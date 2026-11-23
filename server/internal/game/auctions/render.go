// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"fmt"
	"time"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// judgement is the buyer's view of an auction price: canAfford, levelOk, the quality name and, for equippable
// items, vsEquipped. The items context renders it from a dump node, so the fields stay as it wrote them.
type judgement struct {
	fields map[string]any
	Score  float64 // what the item adds over the worn one (item level plus DPS)
}

func judge(p Price, buyer items.Buyer) *judgement {
	fields := map[string]any{}
	buyer.Judge(fields, snapshot.AsMap(p.Raw), p.MinBuyout, p.HasBuyout)
	if !p.HasRequiredLevel {
		delete(fields, "levelOk") // the required level is unknown: do not claim it is met
	}
	if !p.HasNumbers {
		delete(fields, "vsEquipped") // an item the client had not cached has a slot but no numbers
	}
	return &judgement{fields: fields, Score: items.UpgradeScore(fields)}
}

// View combines the saved prices with the character's items: what is worth listing
// (sellOnAH, biggest profit first), which prices are missing, and the player's own auctions.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	return Analyse(s, q, fresh).Render()
}

func (r Report) Render() map[string]any {
	if !r.Saved {
		return map[string]any{"note": "No auction house prices. In game, open the auction house and search for the items you want to check (the addon saves what the results window shows), then type /mcp reload. Prices accumulate between visits."}
	}
	rows := make([]any, len(r.Rows))
	for i, row := range r.Rows {
		rows[i] = row.render()
	}
	sell := make([]any, len(r.Sales))
	for i, sale := range r.Sales {
		sell[i] = sale.render()
	}
	out := map[string]any{
		"prices":      rows,
		"count":       len(rows),
		"sellOnAH":    sell,
		"assumptions": fmt.Sprintf("Profit is computed from the lowest buyout price per unit minus the %.0f%% auction house cut. The listing deposit is lost if the item does not sell and is not counted here. Bound (Soulbound) and quest items cannot be listed.", AuctionCut*100),
	}
	if note := r.detailsNote(); note != "" {
		out["detailsNote"] = note
	}
	if r.Equippable {
		out["sortedBy"] = "best upgrade first (item level plus DPS gained over the equipped item)"
	}
	if r.AuctionTotal > 0 {
		out["sellOnAHSummary"] = fmt.Sprintf("Listing the items with advice=auction: about %s after the cut instead of %s from a vendor.", snapshot.Money(r.AuctionTotal), snapshot.Money(r.VendorTotal))
	}
	if len(r.Unpriced) > 0 {
		out["checkPrices"] = r.Unpriced
		out["checkPricesNote"] = "These items (Uncommon quality and better) are missing from the saved prices. Search for them in the auction house, then /mcp reload."
	}
	if r.Partial != nil {
		out["lastSearchPartial"] = fmt.Sprintf("The last search had %v results, and a page of %v was saved. The lowest price may be on another page; sort the results by price.", r.Partial.Total, r.Partial.Shown)
	}
	if r.HasVisit {
		out["lastVisit"] = snapshot.FormatAge(r.LastVisitAge) + " ago"
	}
	if r.Mine != nil {
		out["myAuctions"] = r.Mine.render()
	}
	return out
}

func (r Report) detailsNote() string {
	const how = "Item details (type, level, stats, DPS) are saved from addon 0.13.0, only for gear and only when the client already holds the item data. Update the addon, browse the auction house for the items again and type /mcp reload."
	switch r.Gap {
	case DetailsNone:
		return "No saved price has item details, so gear cannot be compared. " + how
	case DetailsPartial:
		return fmt.Sprintf("%d saved prices have no item details and are left out of this answer (non-gear never has them; gear needs addon 0.13.0 and a browse with the item data cached). %s", r.NoDetails, how)
	}
	return ""
}

// render describes one price: amounts, age, the trend over recent visits, the buyer's judgement and the stock held.
func (r PriceRow) render() map[string]any {
	e := map[string]any{"name": r.Name}
	passThrough(e, r.Passed)
	if r.HasBuyout {
		e["minBuyoutPerUnit"] = snapshot.Money(r.MinBuyout).String()
	}
	if r.HasBid {
		e["minBidPerUnit"] = snapshot.Money(r.MinBid).String()
	}
	if r.HasSeenAge {
		e["seenAgo"] = snapshot.FormatAge(r.SeenAge)
		if r.Stale {
			e["stale"] = true
		}
	}
	if t := r.Trend; t != nil {
		e["trend"] = fmt.Sprintf("%+.0f%% over %d visits (%s -> %s)", t.ChangePercent, t.Visits, snapshot.Money(t.First), snapshot.Money(t.Last))
	}
	if r.Judgement != nil {
		passThrough(e, r.Details)
		snapshot.Merge(e, r.Judgement.fields)
	}
	if o := r.Owned; o != nil {
		e["have"] = o.Count
		e["where"] = o.Where()
		if o.SellPrice > 0 {
			e["vendorPerUnit"] = snapshot.Money(o.SellPrice).String()
		}
	}
	return e
}

func passThrough(e map[string]any, details []Detail) {
	for _, d := range details {
		e[d.Key] = d.Value
	}
}

func (sa Sale) render() map[string]any {
	e := map[string]any{
		"name":           sa.Name,
		"have":           sa.Have,
		"where":          sa.Where,
		"auctionPerUnit": snapshot.Money(sa.Buyout).String(),
		"afterCutTotal":  snapshot.Money(sa.AuctionNet()).String(),
		"vendorTotal":    snapshot.Money(sa.VendorTotal()).String(),
		"advice":         string(sa.Advice),
	}
	if sa.Stale {
		e["priceAge"] = snapshot.FormatAge(sa.SeenAge)
	}
	return e
}

func (m MyAuctions) render() map[string]any {
	list := []any{}
	for _, a := range m.Auctions {
		list = append(list, a.render())
	}
	out := map[string]any{"auctions": list, "count": len(list)}
	if m.Sold > 0 {
		out["sold"] = m.Sold
	}
	if m.ActiveBuyout > 0 {
		out["totalBuyout"] = snapshot.Money(m.ActiveBuyout).String()
	}
	if m.HasChecked {
		out["checkedAgo"] = snapshot.FormatAge(m.CheckedAge)
	}
	return out
}

func (a OwnAuction) render() map[string]any {
	e := map[string]any{"name": a.Name, "count": a.Count}
	if a.Buyout > 0 {
		e["buyout"] = snapshot.Money(a.Buyout).String()
	}
	if a.Bid > 0 {
		e["bid"] = snapshot.Money(a.Bid).String()
	}
	if a.Sold {
		e["sold"] = true
	}
	if a.HasTimeLeft {
		e["timeLeft"] = snapshot.FormatSeconds(a.TimeLeft)
	}
	return e
}

// EndingSoon describes the endings: "Copper Ore x10 (1 h 30 min left)".
func EndingSoon(s snapshot.Snapshot, fresh snapshot.Freshness, within time.Duration) []string {
	var out []string
	for _, e := range EndingAuctions(s, fresh, within) {
		out = append(out, fmt.Sprintf("%s x%d (%s)", e.Name, e.Count, e.leftLabel()))
	}
	return out
}

func (e Ending) leftLabel() string {
	if e.Left <= 0 {
		return "may have already expired"
	}
	return snapshot.FormatAge(e.Left) + " left"
}
