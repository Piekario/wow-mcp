// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package items covers items: bags, bank, vendor offer, comparison
// with equipped gear and searching for an item across all characters on the account.
package items

import "wowmcp/internal/game/itemkind"

// Item is one item from bags, the bank, a vendor, buyback, mail or the equipped gear, read once from the dump.
// Classification (class, binding, quest) comes from the embedded itemkind.Item.
type Item struct {
	itemkind.Item
	ID            int64
	HasID         bool
	Count         int64 // units in the stack; an item without a count is one unit
	Quality       int64
	HasQuality    bool
	SellPrice     int64 // vendor value of one unit
	Price         int64 // priceCopper: the vendor price for Stack units, or the buyback price of the whole batch
	HasPrice      bool
	Stack         int64
	EquipLoc      string
	RequiredLevel int64
	Unusable      bool   // the addon marked the item as unusable for the character
	Slot          string // the equipment slot of a worn item
	HasSlot       bool
	Weapon        WeaponNumbers
	Stats         map[string]float64 // stat key (token, or the client's label in old dumps) -> value
	ItemLevel     float64
	HasItemLevel  bool
	HasTooltip    bool
	HasStatTokens bool
	tooltipLines  int
	present       bool
	raw           record
}

// WeaponNumbers are the numeric weapon fields of addon 0.10.0; zero when the dump has none.
type WeaponNumbers struct {
	DamageMin, DamageMax float64
	Speed                float64
	HasSpeed             bool
	DPS                  float64
	HasDPS               bool
}

func (i Item) QualityName() string {
	if !i.HasQuality {
		return ""
	}
	name, _ := itemkind.QualityName(i.Quality)
	return name
}

func (i Item) IsJunk() bool { return i.HasQuality && i.Quality == 0 }

// SellValue is the vendor value of the whole stack.
func (i Item) SellValue() int64 { return i.SellPrice * i.Count }

func (i Item) LevelOk(level int64) bool { return i.RequiredLevel <= level }
