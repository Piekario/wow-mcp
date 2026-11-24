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
	if m := snapshot.AsMap(sec["lastAttempt"]); m != nil {
		out.LastAttempt = readAttempt(m)
	}
	return out
}

func readAttempt(m map[string]any) *Attempt {
	a := &Attempt{At: snapshot.Captured{}, InInst: m["inInstance"] == true}
	a.At.At, a.At.Known = m["at"].(int64)
	a.Query, _ = m["query"].(string)
	a.Outcome, _ = m["outcome"].(string)
	a.Error, _ = m["error"].(string)
	a.Hint, _ = m["hint"].(string)
	a.Offered, _ = snapshot.AsInt(m["instancesInJournal"])
	a.Tiers, _ = snapshot.AsInt(m["tiers"])
	a.ItemsRead, _ = snapshot.AsInt(m["itemsRead"])
	for _, raw := range snapshot.AsList(m["instanceNames"]) {
		if name, ok := raw.(string); ok {
			a.Names = append(a.Names, name)
		}
	}
	return a
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
