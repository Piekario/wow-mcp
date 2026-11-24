// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

const twoHandEquipLoc = "INVTYPE_2HWEAPON"

// slotsFor maps an item's equipLoc to the addon's equipment slots (names from EQUIPMENT_SLOTS).
func slotsFor(equipLoc string) []string {
	switch equipLoc {
	case "INVTYPE_HEAD":
		return []string{"head"}
	case "INVTYPE_NECK":
		return []string{"neck"}
	case "INVTYPE_SHOULDER":
		return []string{"shoulder"}
	case "INVTYPE_BODY":
		return []string{"shirt"}
	case "INVTYPE_CHEST", "INVTYPE_ROBE":
		return []string{"chest"}
	case "INVTYPE_WAIST":
		return []string{"waist"}
	case "INVTYPE_LEGS":
		return []string{"legs"}
	case "INVTYPE_FEET":
		return []string{"feet"}
	case "INVTYPE_WRIST":
		return []string{"wrist"}
	case "INVTYPE_HAND":
		return []string{"hands"}
	case "INVTYPE_FINGER":
		return []string{"finger1", "finger2"}
	case "INVTYPE_TRINKET":
		return []string{"trinket1", "trinket2"}
	case "INVTYPE_CLOAK":
		return []string{"back"}
	case "INVTYPE_WEAPON":
		return []string{"mainHand", "offHand"}
	case twoHandEquipLoc, "INVTYPE_WEAPONMAINHAND":
		return []string{"mainHand"}
	case "INVTYPE_WEAPONOFFHAND", "INVTYPE_SHIELD", "INVTYPE_HOLDABLE":
		return []string{"offHand"}
	case "INVTYPE_RANGED", "INVTYPE_RANGEDRIGHT", "INVTYPE_THROWN", "INVTYPE_RELIC":
		return []string{"ranged"}
	case "INVTYPE_TABARD":
		return []string{"tabard"}
	}
	return nil
}

func (i Item) Slots() []string { return slotsFor(i.EquipLoc) }

func (i Item) Equippable() bool { return len(i.Slots()) > 0 }

// TwoHanded: such a weapon also removes the off-hand item.
func (i Item) TwoHanded() bool { return i.EquipLoc == twoHandEquipLoc }

// GearSlots are the equipment slots gear is compared in (names from the addon's EQUIPMENT_SLOTS), in the order of
// the character sheet.
var GearSlots = []string{"head", "neck", "shoulder", "back", "chest", "wrist", "hands", "waist", "legs", "feet",
	"finger1", "finger2", "trinket1", "trinket2", "mainHand", "offHand", "ranged"}
