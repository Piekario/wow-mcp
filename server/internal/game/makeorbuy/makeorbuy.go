// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package makeorbuy compares the cost of crafting an item with its auction price:
// reagents are priced at the saved auction house prices (or the vendor price
// when it is cheaper), the finished item at its auction price. A missing price is never zero:
// such a recipe gets the verdict "unknown" and a list of missing prices.
package makeorbuy

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/snapshot"
)

const (
	VerdictMake    = "make"
	VerdictBuy     = "buy"
	VerdictUnknown = "unknown"
)

// A price older than this is marked as stale.
const priceStaleAfter = 48 * time.Hour

const defaultLimit = 50

// Query is the input of wow_make_or_buy.
type Query struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Profession string `json:"profession,omitempty" jsonschema:"profession name or a fragment of it; empty = all saved"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of a recipe or item name"`
	Sort       string `json:"sort,omitempty" jsonschema:"margin (default, biggest saving first) or name"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max recipes in the response; default 50"`
}

// price is a known price per unit.
type price struct {
	copper  int64
	source  string // "auction" or "vendor:<npc>"
	seenAt  int64  // time the auction price was saved (0 = unknown)
	hasSeen bool
}

// priceBook holds auction prices (by name and ID) and vendor prices (by name, lowercase).
type priceBook struct {
	auctionByName map[string]price
	auctionByID   map[int64]price
	vendor        map[string]price
}

func buildBook(s snapshot.Snapshot) priceBook {
	b := priceBook{auctionByName: map[string]price{}, auctionByID: map[int64]price{}, vendor: map[string]price{}}
	prices := snapshot.AsMap(s.Map("auctions")["prices"])
	for _, name := range slices.Sorted(maps.Keys(prices)) {
		p := snapshot.AsMap(prices[name])
		buyout, ok := snapshot.AsInt(p["minBuyout"])
		if !ok || buyout <= 0 {
			continue
		}
		pr := price{copper: buyout, source: "auction"}
		pr.seenAt, pr.hasSeen = snapshot.AsInt(p["seenAt"])
		b.auctionByName[strings.ToLower(name)] = pr
		if id, ok := snapshot.AsInt(p["id"]); ok {
			b.auctionByID[id] = pr
		}
	}
	for _, v := range s.Visits("vendor", "vendors") {
		for _, raw := range snapshot.AsList(v.Section["items"]) {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			total, ok := snapshot.AsInt(item["priceCopper"])
			if name == "" || !ok || total <= 0 {
				continue
			}
			stack, _ := snapshot.AsInt(item["stack"])
			if stack <= 0 {
				stack = 1
			}
			key := strings.ToLower(name)
			unit := (total + stack - 1) / stack
			if old, seen := b.vendor[key]; !seen || unit < old.copper {
				b.vendor[key] = price{copper: unit, source: "vendor:" + v.Npc}
			}
		}
	}
	return b
}

// reagentPrice picks the cheaper of the known reagent prices.
func (b priceBook) reagentPrice(name string) (price, bool) {
	key := strings.ToLower(name)
	a, hasA := b.auctionByName[key]
	v, hasV := b.vendor[key]
	switch {
	case hasA && hasV:
		if v.copper <= a.copper {
			return v, true
		}
		return a, true
	case hasA:
		return a, true
	case hasV:
		return v, true
	}
	return price{}, false
}

// productPrice is the auction price of the finished item (by recipe name or item ID).
func (b priceBook) productPrice(name string, itemID int64) (price, bool) {
	if p, ok := b.auctionByName[strings.ToLower(name)]; ok {
		return p, true
	}
	if itemID > 0 {
		p, ok := b.auctionByID[itemID]
		return p, ok
	}
	return price{}, false
}

type result struct {
	view   map[string]any
	margin int64
	known  bool
}

// evaluate prices one recipe.
func evaluate(prof string, r map[string]any, b priceBook, fresh snapshot.Freshness) (result, bool) {
	reagents := snapshot.AsList(r["reagents"])
	name, _ := r["name"].(string)
	if len(reagents) == 0 || name == "" {
		return result{}, false
	}
	var cost, partial int64
	var lines []any
	var missing, stale []string
	complete := true
	for _, rr := range reagents {
		re := snapshot.AsMap(rr)
		rn := snapshot.ReagentName(re)
		need, _ := snapshot.AsInt(re["need"])
		if need <= 0 {
			need = 1
		}
		line := map[string]any{"name": rn, "need": need}
		p, ok := b.reagentPrice(rn)
		if !ok {
			line["unitPrice"] = nil
			missing = append(missing, rn)
			complete = false
			lines = append(lines, line)
			continue
		}
		line["unitPrice"], line["unitPriceCopper"], line["source"] = snapshot.Money(p.copper).String(), p.copper, p.source
		line["lineTotal"] = snapshot.Money(p.copper * need).String()
		partial += p.copper * need
		cost += p.copper * need
		if note := ageNote(p, fresh); note != "" {
			line["seenAgo"] = note
			if staleAge(p, fresh) {
				stale = append(stale, fmt.Sprintf("%s (%s)", rn, note))
			}
		}
		lines = append(lines, line)
	}
	itemID, _ := snapshot.AsInt(r["itemId"])
	prod, hasProd := b.productPrice(name, itemID)
	v := map[string]any{"name": name, "profession": prof, "reagents": lines}
	if itemID > 0 {
		v["itemId"] = itemID
	}
	if complete {
		v["reagentCost"], v["reagentCostCopper"] = snapshot.Money(cost).String(), cost
	} else if partial > 0 {
		v["knownPartOfCost"] = snapshot.Money(partial).String() + " (incomplete: prices of some reagents are missing)"
	}
	if hasProd {
		v["productPrice"], v["productPriceCopper"] = snapshot.Money(prod.copper).String(), prod.copper
		if note := ageNote(prod, fresh); note != "" {
			v["productSeenAgo"] = note
			if staleAge(prod, fresh) {
				stale = append(stale, fmt.Sprintf("%s (finished item, %s)", name, note))
			}
		}
	} else {
		missing = append(missing, name+" (finished item)")
	}
	res := result{view: v}
	if complete && hasProd {
		res.known, res.margin = true, prod.copper-cost
		v["margin"], v["marginCopper"] = snapshot.Money(res.margin).Signed(), res.margin
		afterCut := int64(float64(prod.copper)*(1-auctions.AuctionCut)) - cost
		v["marginAfterCut"] = snapshot.Money(afterCut).Signed()
		v["verdict"] = VerdictBuy
		if res.margin > 0 {
			v["verdict"] = VerdictMake
		}
	} else {
		v["verdict"] = VerdictUnknown
	}
	if len(missing) > 0 {
		v["missingPrices"] = missing
	}
	if len(stale) > 0 {
		v["stalePrices"] = stale
	}
	return res, true
}

func staleAge(p price, fresh snapshot.Freshness) bool {
	return p.hasSeen && fresh.Age(p.seenAt) >= priceStaleAfter
}

func ageNote(p price, fresh snapshot.Freshness) string {
	if !p.hasSeen {
		return ""
	}
	return snapshot.FormatAge(fresh.Age(p.seenAt))
}

// View compares the crafting cost with the finished item's price for the character's recipes.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	all := s.Map("recipes")
	if len(all) == 0 {
		return map[string]any{"note": "No saved recipes. Open the profession window in game (e.g. Leatherworking), then type /mcp reload. Each profession is saved separately."}
	}
	book := buildBook(s)
	var hints []string
	if len(book.auctionByName) == 0 {
		hints = append(hints, "No auction house prices. Open the auction house, search for reagents and finished items, then /mcp reload.")
	}
	query, profQuery := strings.ToLower(q.Query), strings.ToLower(q.Profession)
	var results []result
	for _, prof := range slices.Sorted(maps.Keys(all)) {
		if profQuery != "" && !strings.Contains(strings.ToLower(prof), profQuery) {
			continue
		}
		for _, raw := range snapshot.AsList(snapshot.AsMap(all[prof])["recipes"]) {
			r := snapshot.AsMap(raw)
			if n, _ := r["name"].(string); query != "" && !strings.Contains(strings.ToLower(n), query) {
				continue
			}
			if res, ok := evaluate(prof, r, book, fresh); ok {
				results = append(results, res)
			}
		}
	}
	byName := q.Sort == "name"
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if byName {
			return fmt.Sprint(a.view["name"]) < fmt.Sprint(b.view["name"])
		}
		if a.known != b.known {
			return a.known
		}
		return a.margin > b.margin
	})
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	counts := map[string]int{}
	for _, r := range results {
		counts[r.view["verdict"].(string)]++
	}
	shown := results
	if len(shown) > limit {
		shown = shown[:limit]
	}
	views := make([]any, len(shown))
	for i, r := range shown {
		views[i] = r.view
	}
	out := map[string]any{
		"recipes": views, "shown": len(views), "total": len(results), "byVerdict": counts,
		"assumptions": "A recipe yields 1 unit of the item with the same name (the server does not know the number of units per craft). A reagent is priced at the lowest buyout price per unit from the auction house, or at the remembered vendor's price if lower; it does not account for what the character already has in bags. margin = finished item price minus reagent cost; marginAfterCut subtracts " + fmt.Sprintf("%.0f%%", auctions.AuctionCut*100) + " auction cut when you want to sell it. A missing price is never counted as zero: then verdict=unknown and missingPrices. Prices older than 48 h are in stalePrices.",
	}
	if len(shown) < len(results) {
		out["truncated"] = true
	}
	if len(hints) > 0 {
		out["hint"] = hints
	}
	if at, ok := snapshot.AsInt(s.Map("auctions")["lastVisitAt"]); ok {
		out["auctionLastVisit"] = snapshot.FormatAge(fresh.Age(at)) + " ago"
	}
	return out
}
