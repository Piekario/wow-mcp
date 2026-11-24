// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package auctions covers the auction house. The addon saves prices from search results
// the player opened in the auction window (auctions.prices, key = item name), and the
// player's own auctions (auctions.owned). Here we combine that with bags and bank:
// what is worth listing and what to hand to a vendor.
package auctions

import (
	"strings"
	"time"

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

// Detail is a dump field passed through to the answer unchanged.
type Detail struct {
	Key   string
	Value any
}

// Price is the lowest price saved for one item. Raw is the dump node: the buyer's judgement of the item lives in
// the items context, which reads items from dump nodes.
type Price struct {
	Key              string // the item name the addon saved the price under
	Name             any
	Passed           []Detail // id, quantity, listings, source
	MinBuyout        int64
	HasBuyout        bool
	MinBid           int64
	HasBid           bool
	SeenAt           int64
	HasSeenAt        bool
	Buyouts          []int64 // the minimum buyout of each earlier visit, oldest first
	Details          []Detail
	HasDetails       bool // the addon saved item details (addon 0.13.0, gear it had cached)
	HasRequiredLevel bool
	HasNumbers       bool // item level or stat tokens: without them a comparison would only subtract the worn item
	HasTooltip       bool // addon 0.19.2 read the row's own tooltip, so a random suffix has its stats
	Raw              any
}

// OwnedItem is a character item summed by name across bags and bank. Sell price and quality are the first
// instance's; bound and quest are true when any instance is.
type OwnedItem struct {
	Name           string
	Count          int64
	SellPrice      int64
	Quality        int64
	InBags, InBank bool
	Bound, Quest   bool
}

// Tradable: bound (Soulbound) and quest items cannot be listed.
func (o OwnedItem) Tradable() bool { return !o.Bound && !o.Quest }

// Where is "bags", "bank" or "bags+bank".
func (o OwnedItem) Where() string {
	var where []string
	if o.InBags {
		where = append(where, "bags")
	}
	if o.InBank {
		where = append(where, "bank")
	}
	return strings.Join(where, "+")
}

// OwnAuction is one of the player's own auctions.
type OwnAuction struct {
	Name, Count any
	Units       int64
	Buyout, Bid int64 // 0 when the auction has none
	Sold        bool
	TimeLeft    int64 // seconds, from the moment the list was saved
	HasTimeLeft bool
}

type SearchPartial struct{ Total, Shown any }

// Section is the auctions section: saved prices and the player's own auctions.
type Section struct {
	Present     bool
	Prices      []Price // by item name
	Owned       []OwnAuction
	HasOwned    bool
	OwnedAt     int64
	HasOwnedAt  bool
	LastVisitAt int64
	HasVisit    bool
	Partial     *SearchPartial
}

// HasData is false when neither prices nor own auctions were ever saved.
func (s Section) HasData() bool { return s.Present && (len(s.Prices) > 0 || len(s.Owned) > 0) }

// Advice is what to do with an item that has an auction price.
type Advice string

const (
	AdviceAuction        Advice = "auction"
	AdviceVendorMarginal Advice = "vendor: the auction adds almost nothing"
	AdviceVendor         Advice = "vendor"
)

// Sale is the assessment of one character item: list it or hand it to a vendor.
type Sale struct {
	Name      string
	Have      int64
	Where     string
	Buyout    int64 // lowest buyout per unit
	Net       int64 // per unit after the cut
	SellPrice int64 // vendor price per unit
	Advice    Advice
	Stale     bool
	SeenAge   time.Duration
}

func (sa Sale) WorthAuction() bool { return sa.Advice == AdviceAuction }

// AuctionNet is the value of all units after the cut.
func (sa Sale) AuctionNet() int64 { return sa.Net * sa.Have }

func (sa Sale) VendorTotal() int64 { return sa.SellPrice * sa.Have }

// Gain is the auction profit over the vendor price, for sorting.
func (sa Sale) Gain() int64 { return (sa.Net - sa.SellPrice) * sa.Have }

// assess compares the price after the cut with the vendor price.
func assess(o *OwnedItem, p Price, fresh snapshot.Freshness) Sale {
	net := int64(float64(p.MinBuyout) * (1 - AuctionCut))
	sa := Sale{Name: p.Key, Have: o.Count, Where: o.Where(), Buyout: p.MinBuyout, Net: net, SellPrice: o.SellPrice}
	sa.SeenAge, sa.Stale = ageOf(p, fresh)
	switch {
	case o.SellPrice > 0 && float64(net) < 1.2*float64(o.SellPrice):
		sa.Advice = AdviceVendorMarginal
	case net <= 0:
		sa.Advice = AdviceVendor
	default:
		sa.Advice = AdviceAuction
	}
	return sa
}

// ageOf is how long ago the price was saved; stale when that is over two days.
func ageOf(p Price, fresh snapshot.Freshness) (age time.Duration, stale bool) {
	if !p.HasSeenAt {
		return 0, false
	}
	age = fresh.Age(p.SeenAt)
	return age, age >= priceStaleAfter
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
	owned := readOwnedItems(s)
	for _, p := range readSection(s).Prices {
		o := owned[p.Key]
		if o == nil || !p.HasBuyout || !o.Tradable() {
			continue
		}
		sale := assess(o, p, fresh)
		offer := Offer{Name: p.Key, Count: o.Count, AuctionNet: sale.AuctionNet(), VendorTotal: sale.VendorTotal(), WorthAuction: sale.WorthAuction()}
		if sale.Stale {
			offer.PriceAge = snapshot.FormatAge(sale.SeenAge)
		}
		out[p.Key] = offer
	}
	return out
}
