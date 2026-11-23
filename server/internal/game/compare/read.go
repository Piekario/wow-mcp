// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package compare

import (
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

func readCard(name string, s snapshot.Snapshot) Card {
	c := s.Character()
	card := Card{Name: name, Class: c["class"], Race: c["race"], Faction: c["faction"], Zone: c["zone"], SubZone: c["subZone"]}
	card.Level, card.Money = s.LevelAndMoney()
	card.Professions = professions.Known(s)
	card.DumpedAtText, card.HasDumpedAtText = s["dumpedAtText"].(string)
	card.DumpedAt, card.HasDumpedAt = s.DumpedAt()
	card.RestedXP, card.HasRestedXP = snapshot.AsInt(c["restedXp"])
	card.PlayedSeconds, card.HasPlayed = snapshot.AsInt(s.Map("played")["totalSeconds"])
	card.Gear = readGear(s)
	card.AverageItemLevel, card.GearPieces = averageItemLevel(card.Gear)
	return card
}

func readGear(s snapshot.Snapshot) []Piece {
	var gear []Piece
	for _, raw := range s.Equipment() {
		it := snapshot.AsMap(raw)
		p := Piece{Name: it["name"]}
		p.Slot, p.HasSlot = it["slot"].(string)
		p.ItemLevel, p.HasItemLevel = snapshot.AsInt(it["itemLevel"])
		p.Quality, p.HasQuality = snapshot.AsInt(it["quality"])
		gear = append(gear, p)
	}
	return gear
}
