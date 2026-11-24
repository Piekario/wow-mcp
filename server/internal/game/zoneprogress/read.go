// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package zoneprogress

import "wowmcp/internal/snapshot"

// readData turns the zoneQuests section and the facts the rules need (completed quests, the log, the level) into types.
func readData(s snapshot.Snapshot) Data {
	section := s.Map("zoneQuests")
	_, present := s["zoneQuests"]
	_, hasCompleted := s["completedQuests"]
	d := Data{
		Present: present, NoIndex: section["noZoneIndex"] == true,
		HasCompleted: hasCompleted, Completed: idSet(s["completedQuests"]), InLog: map[int64]bool{},
	}
	d.CurrentZone, _ = section["currentZone"].(string)
	d.Level, _ = s.LevelAndMoney()
	player := snapshot.AsMap(section["player"])
	d.Player.RaceMask, _ = snapshot.AsInt(player["raceMask"])
	d.Player.ClassMask, _ = snapshot.AsInt(player["classMask"])
	factions := snapshot.AsMap(section["factionRaceMasks"])
	d.Player.AllianceMask, _ = snapshot.AsInt(factions["Alliance"])
	d.Player.HordeMask, _ = snapshot.AsInt(factions["Horde"])
	for _, raw := range s.List("quests") {
		if id, ok := snapshot.AsInt(snapshot.AsMap(raw)["id"]); ok {
			d.InLog[id] = true
		}
	}
	for _, raw := range snapshot.AsList(section["zones"]) {
		d.Zones = append(d.Zones, readZone(snapshot.AsMap(raw)))
	}
	return d
}

func readZone(m map[string]any) Zone {
	z := Zone{Current: m["current"] == true, Truncated: m["truncated"] == true}
	z.Name, _ = m["name"].(string)
	z.AreaID, _ = snapshot.AsInt(m["areaID"])
	count, _ := snapshot.AsInt(m["questCount"])
	z.QuestCount = int(count)
	for _, raw := range snapshot.AsList(m["quests"]) {
		z.Quests = append(z.Quests, readQuest(snapshot.AsMap(raw)))
	}
	return z
}

func readQuest(m map[string]any) Quest {
	q := Quest{Repeatable: m["repeatable"] == true}
	q.ID, _ = snapshot.AsInt(m["id"])
	q.Name, _ = m["name"].(string)
	q.Level, _ = snapshot.AsInt(m["level"])
	q.RequiredLevel, _ = snapshot.AsInt(m["requiredLevel"])
	q.MaxLevel, _ = snapshot.AsInt(m["maxLevel"])
	q.Races, _ = snapshot.AsInt(m["races"])
	q.Classes, _ = snapshot.AsInt(m["classes"])
	q.Next, _ = snapshot.AsInt(m["next"])
	q.BreadcrumbFor, _ = snapshot.AsInt(m["breadcrumbFor"])
	q.PreAny = ids(m["preAny"])
	q.PreAll = ids(m["preAll"])
	q.ExclusiveTo = ids(m["exclusiveTo"])
	if start := snapshot.AsMap(m["start"]); len(start) > 0 {
		st := &Starter{}
		st.Kind, _ = start["kind"].(string)
		st.Name, _ = start["name"].(string)
		st.Zone, _ = start["zone"].(string)
		x, okX := snapshot.AsFloat(start["x"])
		y, okY := snapshot.AsFloat(start["y"])
		st.X, st.Y, st.HasXY = x, y, okX && okY
		q.Start = st
	}
	return q
}

func ids(raw any) []int64 {
	var out []int64
	for _, v := range snapshot.AsList(raw) {
		if id, ok := snapshot.AsInt(v); ok {
			out = append(out, id)
		}
	}
	return out
}

func idSet(raw any) map[int64]bool {
	set := map[int64]bool{}
	for _, id := range ids(raw) {
		set[id] = true
	}
	return set
}
