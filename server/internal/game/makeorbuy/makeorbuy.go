// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package makeorbuy compares the cost of crafting an item with its auction price:
// reagents are priced at the saved auction house prices (or the vendor price
// when it is cheaper), the finished item at its auction price. A missing price is never zero:
// such a recipe gets the verdict "unknown" and a list of missing prices.
package makeorbuy

import (
	"sort"
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

type Reagent struct {
	Name string
	Need int64
}

// Craft is a recipe to price. The finished item is assumed to be one unit with the recipe's name.
type Craft struct {
	Profession string
	Name       string
	ItemID     int64 // 0 when the recipe does not say
	Reagents   []Reagent
}

// Quote is a known price per unit and how old it is.
type Quote struct {
	Copper int64
	Source string // "auction" or "vendor:<npc>"
	Age    time.Duration
	HasAge bool
}

func (q Quote) Stale() bool { return q.HasAge && q.Age >= priceStaleAfter }

// ReagentLine is one reagent of a craft; Price is nil when no price is known.
type ReagentLine struct {
	Reagent
	Price *Quote
}

func (l ReagentLine) Total() int64 { return l.Price.Copper * l.Need }

// MissingPrice names something with no price; Finished is the crafted item itself.
type MissingPrice struct {
	Name     string
	Finished bool
}

type StalePrice struct {
	Name     string
	Age      time.Duration
	Finished bool
}

// Evaluation is the price of one craft against the price of the finished item.
type Evaluation struct {
	Craft
	Lines   []ReagentLine
	Product *Quote
	Missing []MissingPrice
	Stale   []StalePrice
	Cost    int64 // of every reagent, when Complete; otherwise the part that has a price
	Margin  int64 // the finished item's price minus Cost, when Known
}

// Complete: every reagent has a price.
func (e Evaluation) Complete() bool {
	for _, l := range e.Lines {
		if l.Price == nil {
			return false
		}
	}
	return true
}

// Known: the margin can be told, because every reagent and the finished item have a price.
func (e Evaluation) Known() bool { return e.Complete() && e.Product != nil }

func (e Evaluation) Verdict() string {
	switch {
	case !e.Known():
		return VerdictUnknown
	case e.Margin > 0:
		return VerdictMake
	}
	return VerdictBuy
}

// MarginAfterCut is the margin once the auction house takes its cut of the sale.
func (e Evaluation) MarginAfterCut() int64 {
	return int64(float64(e.Product.Copper)*(1-auctions.AuctionCut)) - e.Cost
}

// Report is the evaluated crafts, limited to what was asked for.
type Report struct {
	Saved           bool
	Evaluations     []Evaluation
	Total           int // evaluated, before the limit
	ByVerdict       map[string]int
	NoAuctionPrices bool
	LastVisitAge    time.Duration
	HasVisit        bool
}

func (r Report) Truncated() bool { return len(r.Evaluations) < r.Total }

// Analyse compares the crafting cost with the finished item's price for the character's recipes.
func Analyse(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) Report {
	crafts, saved := readCrafts(s, q, fresh)
	if !saved {
		return Report{}
	}
	m := readMarket(s)
	book := newBook(m)
	r := Report{Saved: true, NoAuctionPrices: len(m.Auction) == 0, ByVerdict: map[string]int{}}
	var all []Evaluation
	for _, c := range crafts {
		e := evaluate(c, book, fresh)
		r.ByVerdict[e.Verdict()]++
		all = append(all, e)
	}
	sortEvaluations(all, q.Sort == "name")
	r.Total = len(all)
	r.Evaluations = all[:min(len(all), limitOf(q))]
	if m.HasVisit {
		r.LastVisitAge, r.HasVisit = fresh.Age(m.LastVisitAt), true
	}
	return r
}

func limitOf(q Query) int {
	if q.Limit <= 0 {
		return defaultLimit
	}
	return q.Limit
}

// sortEvaluations puts the crafts with a known margin first, the biggest saving on top; byName sorts by name instead.
func sortEvaluations(list []Evaluation, byName bool) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if byName {
			return a.Name < b.Name
		}
		if a.Known() != b.Known() {
			return a.Known()
		}
		return a.Margin > b.Margin
	})
}

// evaluate prices one craft.
func evaluate(c Craft, book priceBook, fresh snapshot.Freshness) Evaluation {
	e := Evaluation{Craft: c}
	for _, re := range c.Reagents {
		line := ReagentLine{Reagent: re}
		if p, ok := book.reagentPrice(re.Name); ok {
			q := p.quote(fresh)
			line.Price = &q
			e.Cost += line.Total()
			if q.Stale() {
				e.Stale = append(e.Stale, StalePrice{Name: re.Name, Age: q.Age})
			}
		} else {
			e.Missing = append(e.Missing, MissingPrice{Name: re.Name})
		}
		e.Lines = append(e.Lines, line)
	}
	if p, ok := book.productPrice(c.Name, c.ItemID); ok {
		q := p.quote(fresh)
		e.Product = &q
		if q.Stale() {
			e.Stale = append(e.Stale, StalePrice{Name: c.Name, Age: q.Age, Finished: true})
		}
	} else {
		e.Missing = append(e.Missing, MissingPrice{Name: c.Name, Finished: true})
	}
	if e.Known() {
		e.Margin = e.Product.Copper - e.Cost
	}
	return e
}
