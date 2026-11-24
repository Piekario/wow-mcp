// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeonloot

import (
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

func readSection(s snapshot.Snapshot) Section {
	sec := s.Map("dungeonLoot")
	if sec == nil {
		return Section{}
	}
	out := Section{Present: true}
	out.Unavailable, _ = sec["unavailable"].(string)
	for _, raw := range snapshot.AsList(sec["instances"]) {
		out.Dungeons = append(out.Dungeons, readDungeon(snapshot.AsMap(raw)))
	}
	return out
}

func readDungeon(m map[string]any) Dungeon {
	d := Dungeon{Captured: snapshot.CapturedOf(m), Raid: m["raid"] == true, Truncated: m["truncated"] == true}
	d.Name, _ = m["name"].(string)
	d.JournalID, _ = snapshot.AsInt(m["journalInstanceID"])
	d.Min, _ = snapshot.AsInt(m["minLevel"])
	d.Max, _ = snapshot.AsInt(m["maxLevel"])
	for _, raw := range snapshot.AsList(m["encounters"]) {
		if name, ok := raw.(string); ok {
			d.Encounters = append(d.Encounters, name)
		}
	}
	for _, raw := range snapshot.AsList(m["items"]) {
		it := snapshot.AsMap(raw)
		l := Loot{Item: items.Read(it), Pending: it["pending"] == true}
		l.Name, _ = it["name"].(string)
		l.ID, _ = snapshot.AsInt(it["id"])
		l.Encounter, _ = it["encounter"].(string)
		d.Items = append(d.Items, l)
	}
	return d
}

func readCharacter(s snapshot.Snapshot) Character {
	c := Character{}
	c.ClassToken, _ = s.ClassAndRace()
	c.Class, _ = s.Character()["class"].(string)
	c.Level, _ = s.LevelAndMoney()
	return c
}
