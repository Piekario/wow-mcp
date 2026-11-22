// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"

	"wowmcp/internal/snapshot"
)

// Buyer is what a character needs to judge an offer (a vendor item or an auction house price):
// level, money and the equipped items. The same judgement backs wow_vendor and wow_auctions.
type Buyer struct {
	Level, Money int64
	Equipped     []any
}

// NewBuyer reads the buyer from a snapshot.
func NewBuyer(s snapshot.Snapshot) Buyer {
	level, money := s.LevelAndMoney()
	return Buyer{Level: level, Money: money, Equipped: s.Equipment()}
}

// OfferFilter holds the filters shared by wow_vendor and wow_auctions.
type OfferFilter struct {
	Affordable bool // only what the buyer can pay now
	Usable     bool // only items whose required level the buyer has
	Equippable bool // only equippable things
}

// Admits says whether the offer passes the filters. An unknown price or an unknown required level never excludes.
func (b Buyer) Admits(item map[string]any, price int64, hasPrice bool, f OfferFilter) bool {
	if f.Affordable && hasPrice && price > b.Money {
		return false
	}
	if req, _ := snapshot.AsInt(item["requiredLevel"]); f.Usable && req > b.Level {
		return false
	}
	return !f.Equippable || Equippable(item)
}

// Judge adds to the entry what the buyer wants to know about the offer: canAfford (when there is a price),
// levelOk, the quality name and, for equippable items, vsEquipped: the difference to what is worn.
func (b Buyer) Judge(entry, item map[string]any, price int64, hasPrice bool) {
	if hasPrice {
		entry["canAfford"] = price <= b.Money
	}
	req, _ := snapshot.AsInt(item["requiredLevel"])
	entry["levelOk"] = req <= b.Level
	if name := qualityName(item); name != "" {
		entry["qualityName"] = name
	}
	if Equippable(item) {
		if cmp := Compare(item, b.Equipped); len(cmp) > 0 {
			entry["vsEquipped"] = bestComparison(cmp)
		}
	}
}

// MatchesType says whether the item is of the given type or subtype (e.g. Weapon, Armor, Bow); by class ID where the
// dump has it, otherwise by the type text.
func MatchesType(item map[string]any, kind string) bool {
	return filter{kind: kind}.match(item)
}

// UpgradeScore ranks the vsEquipped comparison of Judge: item level plus DPS gained (the same measure that picks the
// best slot). An entry without a comparison gets -Inf, so it sorts last.
func UpgradeScore(entry map[string]any) float64 {
	vs := snapshot.AsMap(entry["vsEquipped"])
	if vs == nil {
		return math.Inf(-1)
	}
	return comparisonScore(vs)
}

func comparisonScore(c map[string]any) float64 {
	delta, _ := c["delta"].(map[string]float64)
	return delta["Item Level"] + delta["DPS"]
}
