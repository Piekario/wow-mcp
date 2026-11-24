// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import "wowmcp/internal/snapshot"

// FoodItem is a bag item the game client was asked about with C_PetInfo.CanPetEatItem (addon 0.22.0). Count is the
// total over all its stacks.
type FoodItem struct {
	ID     int64
	Name   string
	Count  int64
	CanEat bool
}

// FoodCheck is the client's verdict on the food in the bags. Known is false when the dump has no such data (an older
// addon, or a client without CanPetEatItem): then only BagFood, matched by item subtype, is left.
type FoodCheck struct {
	Known       bool
	CheckedWith string // the argument form the client answered: "itemID" or "itemLink"
	Edible      []FoodItem
	Refused     []FoodItem // food and drink the pet refuses (the addon leaves out Trade Goods it refuses)
	Feedings    int64      // one item per Feed Pet
}

// CheckFood reads the client's verdict from the dump.
func CheckFood(s snapshot.Snapshot) FoodCheck {
	items, checkedWith, known := readFoodCheck(s)
	return checkFood(items, checkedWith, known)
}

func checkFood(items []FoodItem, checkedWith string, known bool) FoodCheck {
	c := FoodCheck{Known: known, CheckedWith: checkedWith}
	for _, it := range items {
		if it.CanEat {
			c.Edible = append(c.Edible, it)
			c.Feedings += it.Count
		} else {
			c.Refused = append(c.Refused, it)
		}
	}
	return c
}
