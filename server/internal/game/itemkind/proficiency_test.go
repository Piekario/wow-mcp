// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

import "testing"

func TestProficiencyFor(t *testing.T) {
	gear := func(class, subclass int64) Item {
		return Item{Class: class, HasClass: true, Subclass: subclass, HasSubclass: true}
	}
	cases := []struct {
		name      string
		class     string
		level     int64
		item      Item
		want      Proficiency
		fromLevel int64
	}{
		{"hunter mail before 40", "HUNTER", 20, gear(ClassArmor, armorMail), ProficientLater, 40},
		{"hunter mail at 40", "HUNTER", 40, gear(ClassArmor, armorMail), Proficient, 0},
		{"mage leather", "MAGE", 60, gear(ClassArmor, armorLeather), NotProficient, 0},
		{"anyone wears a ring", "MAGE", 1, gear(ClassArmor, armorGeneric), Proficient, 0},
		{"priest wand", "PRIEST", 5, gear(ClassWeapon, weaponWand), Proficient, 0},
		{"hunter wand", "HUNTER", 5, gear(ClassWeapon, weaponWand), NotProficient, 0},
		{"rogue shield", "ROGUE", 30, gear(ClassArmor, armorShield), NotProficient, 0},
		{"fishing pole", "PRIEST", 5, gear(ClassWeapon, WeaponFishingPole), Proficient, 0},
		{"not gear", "WARRIOR", 5, gear(ClassConsumable, 1), Proficient, 0},
		{"unknown class", "", 5, gear(ClassArmor, armorPlate), ProficiencyUnknown, 0},
		{"no IDs", "WARRIOR", 5, Item{Type: "Armor"}, ProficiencyUnknown, 0},
	}
	for _, c := range cases {
		got, from := c.item.ProficiencyFor(c.class, c.level)
		if got != c.want || from != c.fromLevel {
			t.Errorf("%s: got %v from %d, want %v from %d", c.name, got, from, c.want, c.fromLevel)
		}
	}
}
