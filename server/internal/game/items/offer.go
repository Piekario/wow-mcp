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
	Equipped     []Item
}

func NewBuyer(s snapshot.Snapshot) Buyer {
	level, money := s.LevelAndMoney()
	return Buyer{Level: level, Money: money, Equipped: readEquipment(s)}
}

// OfferFilter holds the filters shared by wow_vendor and wow_auctions.
type OfferFilter struct {
	Affordable bool // only what the buyer can pay now
	Usable     bool // only items whose required level the buyer has
	Equippable bool // only equippable things
}

// Allows says whether the offer passes the filters. An unknown price or an unknown required level never excludes.
func (b Buyer) Allows(it Item, price int64, hasPrice bool, f OfferFilter) bool {
	if f.Affordable && hasPrice && price > b.Money {
		return false
	}
	if f.Usable && !it.LevelOk(b.Level) {
		return false
	}
	return !f.Equippable || it.Equippable()
}

// Offer is what the buyer wants to know about an item on offer. VsEquipped is the best slot comparison of an
// equippable item, nil otherwise.
type Offer struct {
	HasPrice, CanAfford bool
	LevelOk             bool
	Quality             string
	VsEquipped          *SlotComparison
}

func (b Buyer) Assess(it Item, price int64, hasPrice bool) Offer {
	o := Offer{HasPrice: hasPrice, CanAfford: hasPrice && price <= b.Money, LevelOk: it.LevelOk(b.Level), Quality: it.QualityName()}
	if best, ok := bestSlot(GearOf(b.Equipped).Compare(it)); ok {
		o.VsEquipped = &best
	}
	return o
}

// UpgradeScore ranks offers by their best comparison; an offer without one gets -Inf, so it sorts last.
func (o Offer) UpgradeScore() float64 {
	if o.VsEquipped == nil {
		return math.Inf(-1)
	}
	return o.VsEquipped.UpgradeScore()
}

// UpgradeSlot says whether an equippable item looks like an upgrade over what is equipped in the matching slot
// (higher item level or DPS, or an empty slot) and for which slot; false when it cannot be equipped or the level is
// too low for it. This is only a hint: the player decides about stats.
func (b Buyer) UpgradeSlot(it Item) (string, bool) {
	if !it.Equippable() || !it.LevelOk(b.Level) {
		return "", false
	}
	for _, c := range GearOf(b.Equipped).Compare(it) {
		if c.LooksLikeUpgrade() {
			return c.Slot, true
		}
	}
	return "", false
}
