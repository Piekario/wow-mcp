// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package auctions covers the auction house. The addon saves prices from search results
// the player opened in the auction window (auctions.prices, key = item name), and the
// player's own auctions (auctions.owned). Here we combine that with bags and bank:
// what is worth listing and what to hand to a vendor.
package auctions

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// AuctionCut is the auction house cut on a sale: 5% as in the classic game.
const AuctionCut = 0.05

// A price older than this is marked as stale.
const priceStaleAfter = 48 * time.Hour

// Query holds the wow_auctions filters.
type Query struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of an item name"`
	Mine       bool   `json:"mine,omitempty" jsonschema:"true = only prices of items the character has in bags or bank"`
	Type       string `json:"type,omitempty" jsonschema:"item type or subtype, e.g. Weapon, Armor, Bow (only prices saved with item details, addon 0.13.0)"`
	Affordable bool   `json:"affordable,omitempty" jsonschema:"true = only what the character can pay for now (lowest buyout)"`
	Usable     bool   `json:"usable,omitempty" jsonschema:"true = only items whose required level the character already has"`
	Equippable bool   `json:"equippable,omitempty" jsonschema:"true = only equippable things (weapons, armor, jewelry), the best upgrade first"`
}

// detailKeys are the item details the addon saves with a price (0.13.0, from the client cache only).
var detailKeys = []string{"type", "subType", "classID", "subclassID", "equipLoc", "itemLevel", "requiredLevel", "quality", "sellPrice", "bindType", "dps", "statTokens"}

// hasDetails says whether the price carries item details (the addon saved them only for gear it had cached).
func hasDetails(p map[string]any) bool {
	_, equip := p["equipLoc"]
	_, class := p["classID"]
	return equip || class
}

// judgePrice adds the buyer's view of an auction price: details, canAfford, levelOk and, for equippable items, vsEquipped.
func judgePrice(e, p map[string]any, buyer items.Buyer) {
	for _, k := range detailKeys {
		if v, ok := p[k]; ok {
			e[k] = v
		}
	}
	buyout, hasBuyout := snapshot.AsInt(p["minBuyout"])
	buyer.Judge(e, p, buyout, hasBuyout)
	if _, known := p["requiredLevel"]; !known {
		delete(e, "levelOk") // the required level is unknown: do not claim it is met
	}
	// An item the client had not cached has a slot but no numbers: a comparison would only subtract the worn item.
	_, hasLevel := p["itemLevel"]
	_, hasStats := p["statTokens"]
	if !hasLevel && !hasStats {
		delete(e, "vsEquipped")
	}
}

// ownedItem is a character item summed by name across bags and bank.
type ownedItem struct {
	name             string
	id               int64
	count, sellPrice int64
	quality          int64
	where            map[string]bool
	soulbound, quest bool
}

// tradable: bound (Soulbound) and quest items cannot be listed.
func (o *ownedItem) tradable() bool { return !o.soulbound && !o.quest }

// whereLabel is "bags", "bank" or "bags+bank".
func (o *ownedItem) whereLabel() string {
	var out []string
	for _, w := range []string{"bags", "bank"} {
		if o.where[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, "+")
}

// ownedItems sums items from bags and bank by name.
func ownedItems(s snapshot.Snapshot) map[string]*ownedItem {
	out := map[string]*ownedItem{}
	add := func(where string, items []any) {
		for _, raw := range items {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			o := out[name]
			if o == nil {
				o = &ownedItem{name: name, where: map[string]bool{}}
				o.id, _ = snapshot.AsInt(item["id"])
				o.sellPrice, _ = snapshot.AsInt(item["sellPrice"])
				o.quality, _ = snapshot.AsInt(item["quality"])
				out[name] = o
			}
			o.count += snapshot.ItemCount(item)
			o.where[where] = true
			if itemkind.IsQuestItem(item) {
				o.quest = true
			}
			if itemkind.IsBound(item) {
				o.soulbound = true
			}
		}
	}
	for _, bag := range s.Bags() {
		add("bags", snapshot.AsList(snapshot.AsMap(bag)["items"]))
	}
	for _, c := range s.BankContainers() {
		add("bank", snapshot.AsList(snapshot.AsMap(c)["items"]))
	}
	return out
}

// priceEntry describes one price: amounts, age and the trend over recent visits.
func priceEntry(p map[string]any, fresh snapshot.Freshness) map[string]any {
	e := map[string]any{"name": p["name"]}
	for _, k := range []string{"id", "quantity", "listings", "source"} {
		if v, ok := p[k]; ok {
			e[k] = v
		}
	}
	if v, ok := snapshot.AsInt(p["minBuyout"]); ok {
		e["minBuyoutPerUnit"] = snapshot.Money(v).String()
	}
	if v, ok := snapshot.AsInt(p["minBid"]); ok {
		e["minBidPerUnit"] = snapshot.Money(v).String()
	}
	if at, ok := snapshot.AsInt(p["seenAt"]); ok {
		age := fresh.Age(at)
		e["seenAgo"] = snapshot.FormatAge(age)
		if age >= priceStaleAfter {
			e["stale"] = true
		}
	}
	seen := snapshot.AsList(p["seen"])
	if len(seen) >= 2 {
		first, _ := snapshot.AsInt(snapshot.AsMap(seen[0])["minBuyout"])
		last, _ := snapshot.AsInt(snapshot.AsMap(seen[len(seen)-1])["minBuyout"])
		if first > 0 && last > 0 {
			change := float64(last-first) / float64(first) * 100
			e["trend"] = fmt.Sprintf("%+.0f%% over %d visits (%s -> %s)", change, len(seen), snapshot.Money(first), snapshot.Money(last))
		}
	}
	return e
}

// sellOffer is the assessment of one character item: auction or vendor.
type sellOffer struct {
	fields map[string]any
	net    int64 // per-unit price after the cut
	gain   int64 // auction profit over the vendor price (for sorting)
}

// assess compares the price after the cut with the vendor price; toAuction = advice "auction".
func assess(name string, o *ownedItem, buyout int64, price map[string]any) (offer sellOffer, toAuction bool) {
	net := int64(float64(buyout) * (1 - AuctionCut))
	c := map[string]any{
		"name":           name,
		"have":           o.count,
		"where":          price["where"],
		"auctionPerUnit": snapshot.Money(buyout).String(),
		"afterCutTotal":  snapshot.Money(net * o.count).String(),
		"vendorTotal":    snapshot.Money(o.sellPrice * o.count).String(),
	}
	if price["stale"] == true {
		c["priceAge"] = price["seenAgo"]
	}
	switch {
	case o.sellPrice > 0 && float64(net) < 1.2*float64(o.sellPrice):
		c["advice"] = "vendor: the auction adds almost nothing"
	case net <= 0:
		c["advice"] = "vendor"
	default:
		c["advice"] = "auction"
		toAuction = true
	}
	return sellOffer{fields: c, net: net, gain: (net - o.sellPrice) * o.count}, toAuction
}

// View combines the saved prices with the character's items: what is worth listing
// (sellOnAH, biggest profit first), which prices are missing, and the player's own auctions.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	auctions := s.Map("auctions")
	prices := snapshot.AsMap(auctions["prices"])
	if auctions == nil || (len(prices) == 0 && len(snapshot.AsList(auctions["owned"])) == 0) {
		return map[string]any{"note": "No auction house prices. In game, open the auction house and search for the items you want to check (the addon saves what the results window shows), then type /mcp reload. Prices accumulate between visits."}
	}
	owned := ownedItems(s)
	query := strings.ToLower(q.Query)
	buyer := items.NewBuyer(s)
	only := items.OfferFilter{Affordable: q.Affordable, Usable: q.Usable, Equippable: q.Equippable}
	gearFilter := q.Type != "" || q.Usable || q.Equippable

	list := []any{}
	withDetails, withoutDetails := countDetails(prices)
	var offers []sellOffer
	var auctionTotal, vendorTotal int64
	for _, name := range slices.Sorted(maps.Keys(prices)) {
		p := snapshot.AsMap(prices[name])
		if query != "" && !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		o := owned[name]
		if q.Mine && o == nil {
			continue
		}
		buyout, hasBuyout := snapshot.AsInt(p["minBuyout"])
		if gearFilter && !hasDetails(p) {
			continue // without details we cannot tell the type, level or slot
		}
		if (q.Type != "" && !items.MatchesType(p, q.Type)) || !buyer.Admits(p, buyout, hasBuyout, only) {
			continue
		}
		e := priceEntry(p, fresh)
		if hasDetails(p) {
			judgePrice(e, p, buyer)
		}
		if o != nil {
			e["have"] = o.count
			e["where"] = o.whereLabel()
			if o.sellPrice > 0 {
				e["vendorPerUnit"] = snapshot.Money(o.sellPrice).String()
			}
			if hasBuyout && o.tradable() {
				offer, toAuction := assess(name, o, buyout, e)
				if toAuction {
					auctionTotal += offer.net * o.count
					vendorTotal += o.sellPrice * o.count
				}
				offers = append(offers, offer)
			}
		}
		list = append(list, e)
	}
	if q.Equippable {
		sort.SliceStable(list, func(i, j int) bool {
			return items.UpgradeScore(list[i].(map[string]any)) > items.UpgradeScore(list[j].(map[string]any))
		})
	}
	sort.SliceStable(offers, func(i, j int) bool { return offers[i].gain > offers[j].gain })
	sell := make([]any, len(offers))
	for i, offer := range offers {
		sell[i] = offer.fields
	}

	out := map[string]any{
		"prices":      list,
		"count":       len(list),
		"sellOnAH":    sell,
		"assumptions": fmt.Sprintf("Profit is computed from the lowest buyout price per unit minus the %.0f%% auction house cut. The listing deposit is lost if the item does not sell and is not counted here. Bound (Soulbound) and quest items cannot be listed.", AuctionCut*100),
	}
	if note := detailsNote(withDetails, withoutDetails, gearFilter); note != "" {
		out["detailsNote"] = note
	}
	if q.Equippable {
		out["sortedBy"] = "best upgrade first (item level plus DPS gained over the equipped item)"
	}
	if auctionTotal > 0 {
		out["sellOnAHSummary"] = fmt.Sprintf("Listing the items with advice=auction: about %s after the cut instead of %s from a vendor.", snapshot.Money(auctionTotal), snapshot.Money(vendorTotal))
	}
	if unknown := unpriced(owned, prices); len(unknown) > 0 {
		out["checkPrices"] = unknown
		out["checkPricesNote"] = "These items (Uncommon quality and better) are missing from the saved prices. Search for them in the auction house, then /mcp reload."
	}
	if partial := snapshot.AsMap(auctions["lastSearchPartial"]); partial != nil {
		out["lastSearchPartial"] = fmt.Sprintf("The last search had %v results, and a page of %v was saved. The lowest price may be on another page; sort the results by price.", partial["total"], partial["shown"])
	}
	if at, ok := snapshot.AsInt(auctions["lastVisitAt"]); ok {
		out["lastVisit"] = snapshot.FormatAge(fresh.Age(at)) + " ago"
	}
	if mine := ownedAuctions(auctions, fresh); mine != nil {
		out["myAuctions"] = mine
	}
	return out
}

// countDetails counts the saved prices with and without item details.
func countDetails(prices map[string]any) (with, without int) {
	for _, raw := range prices {
		if hasDetails(snapshot.AsMap(raw)) {
			with++
		} else {
			without++
		}
	}
	return with, without
}

// detailsNote explains missing item details: an old dump (addon before 0.13.0) or a client that had nothing cached.
func detailsNote(with, without int, gearFilter bool) string {
	const how = "Item details (type, level, stats, DPS) are saved from addon 0.13.0, only for gear and only when the client already holds the item data. Update the addon, browse the auction house for the items again and type /mcp reload."
	switch {
	case with == 0 && without > 0:
		return "No saved price has item details, so gear cannot be compared. " + how
	case gearFilter && without > 0:
		return fmt.Sprintf("%d saved prices have no item details and are left out of this answer (non-gear never has them; gear needs addon 0.13.0 and a browse with the item data cached). %s", without, how)
	}
	return ""
}

// unpriced lists items worth checking: Uncommon and better with no price in the dump, excluding bound and quest items.
func unpriced(owned map[string]*ownedItem, prices map[string]any) []string {
	var unknown []string
	for _, name := range slices.Sorted(maps.Keys(owned)) {
		o := owned[name]
		if _, ok := prices[name]; ok || !o.tradable() || o.quality < 2 {
			continue
		}
		unknown = append(unknown, name)
	}
	return unknown
}

// ownedAuctions describes the player's own auctions from the last time the Auctions tab was opened.
func ownedAuctions(auctions map[string]any, fresh snapshot.Freshness) map[string]any {
	list := snapshot.AsList(auctions["owned"])
	if list == nil {
		return nil
	}
	items := []any{}
	var active, sold int64
	for _, raw := range list {
		a := snapshot.AsMap(raw)
		e := map[string]any{"name": a["name"], "count": a["count"]}
		if v, ok := snapshot.AsInt(a["buyout"]); ok && v > 0 {
			e["buyout"] = snapshot.Money(v).String()
			if a["sold"] != true {
				active += v
			}
		}
		if v, ok := snapshot.AsInt(a["bid"]); ok && v > 0 {
			e["bid"] = snapshot.Money(v).String()
		}
		if a["sold"] == true {
			e["sold"] = true
			sold++
		}
		if secs, ok := snapshot.AsInt(a["timeLeftSeconds"]); ok {
			e["timeLeft"] = snapshot.FormatSeconds(secs)
		}
		items = append(items, e)
	}
	out := map[string]any{"auctions": items, "count": len(items)}
	if sold > 0 {
		out["sold"] = sold
	}
	if active > 0 {
		out["totalBuyout"] = snapshot.Money(active).String()
	}
	if at, ok := snapshot.AsInt(auctions["ownedAt"]); ok {
		out["checkedAgo"] = snapshot.FormatAge(fresh.Age(at))
	}
	return out
}

// Offer is the assessment of one character item against the saved auction price and the vendor price.
type Offer struct {
	Name         string
	Count        int64
	AuctionNet   int64  // for all units, after the cut
	VendorTotal  int64  // for all units at a vendor
	PriceAge     string // price age when stale; otherwise empty
	WorthAuction bool   // the price after the cut beats the vendor with a margin (advice = auction)
}

// Offers assesses items from bags and bank that have a saved auction house price and can be listed
// (not bound and not quest items). The key is the item name. The same assessment backs sellOnAH in wow_auctions.
func Offers(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]Offer {
	out := map[string]Offer{}
	prices := snapshot.AsMap(s.Map("auctions")["prices"])
	owned := ownedItems(s)
	for name, raw := range prices {
		p := snapshot.AsMap(raw)
		o := owned[name]
		buyout, ok := snapshot.AsInt(p["minBuyout"])
		if o == nil || !ok || !o.tradable() {
			continue
		}
		offer, toAuction := assess(name, o, buyout, priceEntry(p, fresh))
		age, _ := offer.fields["priceAge"].(string)
		out[name] = Offer{Name: name, Count: o.count, AuctionNet: offer.net * o.count, VendorTotal: o.sellPrice * o.count, PriceAge: age, WorthAuction: toAuction}
	}
	return out
}
