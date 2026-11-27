// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

// Armor subclasses: Enum.ItemArmorSubclass.
const (
	armorGeneric  int64 = 0 // rings, necks, trinkets, off-hand frills: anyone
	armorCloth    int64 = 1
	armorLeather  int64 = 2
	armorMail     int64 = 3
	armorPlate    int64 = 4
	armorCosmetic int64 = 5
	armorShield   int64 = 6
	armorLibram   int64 = 7
	armorIdol     int64 = 8
	armorTotem    int64 = 9
)

// More weapon subclasses (Enum.ItemWeaponSubclass) beyond the ones in itemkind.go.
const (
	weaponAxe1H    int64 = 0
	weaponAxe2H    int64 = 1
	weaponMace1H   int64 = 4
	weaponMace2H   int64 = 5
	weaponPolearm  int64 = 6
	weaponSword1H  int64 = 7
	weaponSword2H  int64 = 8
	weaponStaff    int64 = 10
	weaponFist     int64 = 13
	weaponGeneric  int64 = 14
	weaponDagger   int64 = 15
	weaponThrown   int64 = 16
	weaponWand     int64 = 19
	weaponAnyClass int64 = WeaponFishingPole
)

// Proficiency says whether a class can wear or wield an item.
type Proficiency int

const (
	ProficiencyUnknown Proficiency = iota // no class token or no item class IDs in the dump
	Proficient
	ProficientLater // the class learns it at FromLevel (mail and plate in the classic game)
	NotProficient
)

// classGear is what a class can use in the classic game: armor subclass -> the level the class can wear it from
// (1 = from the start), and the weapon subclasses it can learn. Forever may change it; answers say where it comes from.
type classGear struct {
	armor   map[int64]int64
	weapons []int64
}

var (
	clothOnly    = map[int64]int64{armorCloth: 1}
	clothLeather = map[int64]int64{armorCloth: 1, armorLeather: 1}
)

var classProficiencies = map[string]classGear{
	"WARRIOR": {map[int64]int64{armorCloth: 1, armorLeather: 1, armorMail: 1, armorPlate: 40, armorShield: 1},
		[]int64{weaponAxe1H, weaponAxe2H, weaponBows, weaponGuns, weaponMace1H, weaponMace2H, weaponPolearm, weaponSword1H,
			weaponSword2H, weaponStaff, weaponFist, weaponDagger, weaponThrown, weaponCrossbow}},
	"PALADIN": {map[int64]int64{armorCloth: 1, armorLeather: 1, armorMail: 1, armorPlate: 40, armorShield: 1, armorLibram: 1},
		[]int64{weaponAxe1H, weaponAxe2H, weaponMace1H, weaponMace2H, weaponPolearm, weaponSword1H, weaponSword2H}},
	"HUNTER": {map[int64]int64{armorCloth: 1, armorLeather: 1, armorMail: 40},
		[]int64{weaponAxe1H, weaponAxe2H, weaponBows, weaponGuns, weaponPolearm, weaponSword1H, weaponSword2H, weaponStaff,
			weaponFist, weaponDagger, weaponThrown, weaponCrossbow}},
	"ROGUE": {clothLeather,
		[]int64{weaponBows, weaponGuns, weaponMace1H, weaponSword1H, weaponFist, weaponDagger, weaponThrown, weaponCrossbow}},
	"PRIEST":  {clothOnly, []int64{weaponMace1H, weaponStaff, weaponDagger, weaponWand}},
	"MAGE":    {clothOnly, []int64{weaponSword1H, weaponStaff, weaponDagger, weaponWand}},
	"WARLOCK": {clothOnly, []int64{weaponSword1H, weaponStaff, weaponDagger, weaponWand}},
	"SHAMAN": {map[int64]int64{armorCloth: 1, armorLeather: 1, armorMail: 40, armorShield: 1, armorTotem: 1},
		[]int64{weaponAxe1H, weaponAxe2H, weaponMace1H, weaponMace2H, weaponStaff, weaponFist, weaponDagger}},
	"DRUID": {map[int64]int64{armorCloth: 1, armorLeather: 1, armorIdol: 1},
		[]int64{weaponMace1H, weaponMace2H, weaponStaff, weaponFist, weaponDagger}},
}

// ProficiencyFor says whether a class (its token, e.g. HUNTER) at a level can use the item, and from which level when
// it learns it later. Items that are neither armor nor weapons, and armor anyone wears (rings, cloaks), are Proficient.
func (i Item) ProficiencyFor(classToken string, level int64) (Proficiency, int64) {
	gear, known := classProficiencies[classToken]
	if !known || !i.HasClass || !i.HasSubclass {
		return ProficiencyUnknown, 0
	}
	switch i.Class {
	case ClassArmor:
		if i.Subclass == armorGeneric || i.Subclass == armorCosmetic {
			return Proficient, 0
		}
		from, ok := gear.armor[i.Subclass]
		if !ok {
			return NotProficient, 0
		}
		if from > level {
			return ProficientLater, from
		}
		return Proficient, 0
	case ClassWeapon:
		if i.Subclass == weaponGeneric || i.Subclass == weaponAnyClass {
			return Proficient, 0
		}
		for _, w := range gear.weapons {
			if w == i.Subclass {
				return Proficient, 0
			}
		}
		return NotProficient, 0
	}
	return Proficient, 0
}
