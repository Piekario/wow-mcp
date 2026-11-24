// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package auctions

import (
	"maps"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// Trend is the change of the lowest buyout between the first and the last saved visit.
type Trend struct {
	ChangePercent float64
	Visits        int
	First, Last   int64
	Since         int64 // the first visit kept in the long price history; 0 = only the visits the dump carries
}

// sameVisit mirrors the addon: prices seen within 10 minutes of the previous one belong to one visit.
const sameVisit = 10 * 60

// PriceRow is one price after the filters, with what is known about it and about the character's stock of the item.
type PriceRow struct {
	Price
	SeenAge    time.Duration
	HasSeenAge bool
	Stale      bool
	Trend      *Trend
	Owned      *OwnedItem
	Judgement  *judgement // only for prices with item details
}

// UpgradeScore ranks a row by what it would add over the equipped item; rows with no comparison come last.
func (r PriceRow) UpgradeScore() float64 {
	if r.Judgement == nil {
		return math.Inf(-1)
	}
	return r.Judgement.Score
}

// DetailsGap says why the saved prices cannot all be compared as gear.
type DetailsGap int

const (
	DetailsComplete DetailsGap = iota
	DetailsNone                // no saved price has item details
	DetailsPartial             // some do not, and a gear filter left them out
)

// MyAuctions is the player's own auctions from the last time the Auctions tab was opened.
type MyAuctions struct {
	Auctions     []OwnAuction
	Sold         int64
	ActiveBuyout int64 // the buyout of what is still for sale
	CheckedAge   time.Duration
	HasChecked   bool
}

// Report is the saved prices judged against the character's items. Saved is false when nothing was ever saved.
type Report struct {
	Saved        bool
	Rows         []PriceRow
	Sales        []Sale // biggest auction profit first
	AuctionTotal int64  // after the cut, for the items to list
	VendorTotal  int64  // what a vendor would pay for the same items
	Gap          DetailsGap
	NoDetails    int // saved prices with no item details
	Equippable   bool
	Unpriced     []string
	Partial      *SearchPartial
	LastVisitAge time.Duration
	HasVisit     bool
	Mine         *MyAuctions
	PastError    string // the long price history could not be read: trends use only the dump
}

// Analyse combines the saved prices with the character's items: what is worth listing (biggest profit first),
// which prices are missing, and the player's own auctions.
func Analyse(s snapshot.Snapshot, q Query, fresh snapshot.Freshness, past Past) Report {
	sec := readSection(s)
	if !sec.HasData() {
		return Report{}
	}
	owned := readOwnedItems(s)
	r := Report{Saved: true, Equippable: q.Equippable, Partial: sec.Partial}
	gearFilter := q.Type != "" || q.Usable || q.Equippable
	r.Gap, r.NoDetails = detailsGap(sec.Prices, gearFilter)
	buyer := items.NewBuyer(s)
	only := items.OfferFilter{Affordable: q.Affordable, Usable: q.Usable, Equippable: q.Equippable}
	stored, err := past.observations(sec.Prices)
	if err != nil {
		r.PastError = err.Error()
	}
	for _, p := range sec.Prices {
		if !wanted(p, q, owned[p.Key], gearFilter) || !admitted(p, q, buyer, only) {
			continue
		}
		row := priceRow(p, owned[p.Key], buyer, fresh, stored[p.Key])
		r.Rows = append(r.Rows, row)
		if row.Owned != nil && p.HasBuyout && row.Owned.Tradable() {
			sale := assess(row.Owned, p, fresh)
			if sale.WorthAuction() {
				r.AuctionTotal += sale.AuctionNet()
				r.VendorTotal += sale.VendorTotal()
			}
			r.Sales = append(r.Sales, sale)
		}
	}
	if q.Equippable {
		sort.SliceStable(r.Rows, func(i, j int) bool { return r.Rows[i].UpgradeScore() > r.Rows[j].UpgradeScore() })
	}
	sort.SliceStable(r.Sales, func(i, j int) bool { return r.Sales[i].Gain() > r.Sales[j].Gain() })
	r.Unpriced = unpriced(owned, sec.Prices)
	if sec.HasVisit {
		r.LastVisitAge, r.HasVisit = fresh.Age(sec.LastVisitAt), true
	}
	r.Mine = myAuctions(sec, fresh)
	return r
}

// wanted applies the filters that need no buyer: the name fragment, the character's own items and the gear
// filters, which cannot judge a price without item details.
func wanted(p Price, q Query, o *OwnedItem, gearFilter bool) bool {
	if q.Query != "" && !strings.Contains(strings.ToLower(p.Key), strings.ToLower(q.Query)) {
		return false
	}
	if q.Mine && o == nil {
		return false
	}
	return !gearFilter || p.HasDetails
}

func admitted(p Price, q Query, buyer items.Buyer, only items.OfferFilter) bool {
	raw := snapshot.AsMap(p.Raw)
	if q.Type != "" && !items.MatchesType(raw, q.Type) {
		return false
	}
	return buyer.Admits(raw, p.MinBuyout, p.HasBuyout, only)
}

func priceRow(p Price, o *OwnedItem, buyer items.Buyer, fresh snapshot.Freshness, stored []Observation) PriceRow {
	row := PriceRow{Price: p, Owned: o, Trend: trendOf(p, stored)}
	row.SeenAge, row.Stale = ageOf(p, fresh)
	row.HasSeenAge = p.HasSeenAt
	if p.HasDetails {
		row.Judgement = judge(p, buyer)
	}
	return row
}

// trendOf uses the longer of the visits the dump carries and the visits of the long price history.
func trendOf(p Price, stored []Observation) *Trend {
	series, since := p.Visits, int64(0)
	if visits := visitsOf(stored); len(visits) > len(series) {
		series, since = visits, visits[0].SeenAt
	}
	if len(series) < 2 {
		return nil
	}
	first, last := series[0].MinBuyout, series[len(series)-1].MinBuyout
	if first <= 0 || last <= 0 {
		return nil
	}
	return &Trend{ChangePercent: float64(last-first) / float64(first) * 100, Visits: len(series), First: first, Last: last, Since: since}
}

// visitsOf keeps the last observation of each visit: a dump saved during a visit stored a price the addon later
// replaced with the visit's final one.
func visitsOf(observations []Observation) []Observation {
	var out []Observation
	for _, o := range observations {
		if n := len(out); n > 0 && o.SeenAt-out[n-1].SeenAt < sameVisit {
			out[n-1] = o
			continue
		}
		out = append(out, o)
	}
	return out
}

// observations returns the long price history of the saved prices, by item.
func (past Past) observations(prices []Price) (map[string][]Observation, error) {
	if past.Log == nil || len(prices) == 0 {
		return nil, nil
	}
	keys := make([]string, len(prices))
	for i, p := range prices {
		keys[i] = p.Key
	}
	list, err := past.Log.Prices(past.Realm, keys)
	if err != nil {
		return nil, err
	}
	out := map[string][]Observation{}
	for _, o := range list {
		out[o.Item] = append(out[o.Item], o)
	}
	return out, nil
}

// detailsGap explains missing item details: an old dump (addon before 0.13.0) or a client that had nothing cached.
func detailsGap(prices []Price, gearFilter bool) (DetailsGap, int) {
	without := 0
	for _, p := range prices {
		if !p.HasDetails {
			without++
		}
	}
	switch {
	case without == len(prices) && without > 0:
		return DetailsNone, without
	case gearFilter && without > 0:
		return DetailsPartial, without
	}
	return DetailsComplete, without
}

// unpriced lists items worth checking: Uncommon and better with no price in the dump, excluding bound and quest items.
func unpriced(owned map[string]*OwnedItem, prices []Price) []string {
	priced := map[string]bool{}
	for _, p := range prices {
		priced[p.Key] = true
	}
	var unknown []string
	for _, name := range slices.Sorted(maps.Keys(owned)) {
		o := owned[name]
		if priced[name] || !o.Tradable() || o.Quality < 2 {
			continue
		}
		unknown = append(unknown, name)
	}
	return unknown
}

func myAuctions(sec Section, fresh snapshot.Freshness) *MyAuctions {
	if !sec.HasOwned {
		return nil
	}
	m := &MyAuctions{Auctions: sec.Owned}
	for _, a := range sec.Owned {
		switch {
		case a.Sold:
			m.Sold++
		default:
			m.ActiveBuyout += a.Buyout
		}
	}
	if sec.HasOwnedAt {
		m.CheckedAge, m.HasChecked = fresh.Age(sec.OwnedAt), true
	}
	return m
}
