// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package export

import (
	"wowmcp/internal/game/character"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/game/talents"
	"wowmcp/internal/snapshot"
)

func readCharacter(s snapshot.Snapshot) Character {
	c := s.Character()
	out := Character{Race: c["race"], Class: c["class"], Professions: professions.Known(s), Gear: readGear(s)}
	out.Name, _ = c["name"].(string)
	out.Faction, _ = c["faction"].(string)
	out.Level, out.Money = s.LevelAndMoney()
	tal := s.Map("talents")
	out.TalentSplit, out.MainTree = talents.Split(tal)
	out.LearnedTalents = talents.Learned(tal)
	out.AverageItemLevel, out.HasItemLevel = character.AverageItemLevel(s)
	if pet := s.Map("pet"); pet != nil && pet["exists"] == true {
		out.Pet = &Pet{Name: pet["name"], Family: pet["family"], Level: pet["level"]}
	}
	out.Played, _ = character.Played(s)["total"].(string)
	return out
}

func readGear(s snapshot.Snapshot) map[string]Item {
	gear := map[string]Item{}
	for slot, raw := range snapshot.EquippedBySlot(s.Equipment()) {
		item := Item{Name: raw["name"]}
		item.Quality, item.HasQuality = snapshot.AsInt(raw["quality"])
		item.ItemLevel, item.HasItemLevel = snapshot.AsInt(raw["itemLevel"])
		gear[slot] = item
	}
	return gear
}
