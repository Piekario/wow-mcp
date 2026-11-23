// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "strings"

// SkillLine is a line from the professions section, as far as weapon skills need it. Name, Rank and MaxRank are
// passed through to the answer as the client gave them.
type SkillLine struct {
	ID              int64
	HasID           bool
	Name            any
	Rank, MaxRank   any
	NameText, Group string
}

// WeaponSkill is the character's skill with the compared weapon's type.
type WeaponSkill struct {
	Name, Rank, MaxRank any
}

// weaponSkills maps a weapon subclass (Enum.ItemWeaponSubclass) to its weapon skill line ID (the same IDs the addon
// uses for the weapon skills in the professions section).
var weaponSkills = map[int64]int64{
	0: 44, 1: 172, 2: 45, 3: 46, 4: 54, 5: 160, 6: 229, 7: 43, 8: 55, 10: 136, 13: 473, 15: 173, 16: 176, 18: 226, 19: 228,
}

// weaponSkillFor looks for the character's weapon skill for a weapon: by the weapon subclass and skill line ID
// (addon 0.9.0), otherwise by the English subtype name, e.g. Dagger -> Daggers.
func weaponSkillFor(lines []SkillLine, weapon Item) *WeaponSkill {
	if weapon.HasSubclass && weapon.HasClass {
		if want, known := weaponSkills[weapon.Subclass]; known {
			for _, l := range lines {
				if l.HasID && l.ID == want {
					return &WeaponSkill{Name: l.Name, Rank: l.Rank, MaxRank: l.MaxRank}
				}
			}
		}
	}
	if weapon.SubType == "" {
		return nil
	}
	want := normWeaponName(weapon.SubType)
	for _, l := range lines {
		if !strings.EqualFold(l.Group, "weapon") && !strings.EqualFold(l.Group, "Weapon Skills") {
			continue
		}
		if normWeaponName(l.NameText) == want {
			return &WeaponSkill{Name: l.NameText, Rank: l.Rank, MaxRank: l.MaxRank}
		}
	}
	return nil
}

func normWeaponName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "one-handed ")
	return strings.TrimSuffix(s, "s")
}
