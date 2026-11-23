// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questguide

import "wowmcp/internal/snapshot"

func readPlayer(s snapshot.Snapshot) Player {
	c := s.Character()
	p := Player{}
	p.Zone, _ = c["zone"].(string)
	x, okX := snapshot.AsFloat(c["x"])
	y, okY := snapshot.AsFloat(c["y"])
	p.X, p.Y, p.Known = x, y, okX && okY && p.Zone != ""
	return p
}

func readCompleted(s snapshot.Snapshot) []int64 {
	var ids []int64
	for _, raw := range snapshot.AsList(s["completedQuests"]) {
		if id, ok := snapshot.AsInt(raw); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func readLocation(m map[string]any) Location {
	loc := Location{InInstance: m["inInstance"] == true}
	loc.Zone, _ = m["zone"].(string)
	loc.AreaID, loc.HasAreaID = snapshot.AsInt(m["areaID"])
	x, okX := snapshot.AsFloat(m["x"])
	y, okY := snapshot.AsFloat(m["y"])
	loc.X, loc.Y, loc.HasXY = x, y, okX && okY
	return loc
}

func readTarget(raw any) Target {
	m := snapshot.AsMap(raw)
	t := Target{
		Kind: m["kind"], ID: m["id"], Name: m["name"], Title: m["title"], Text: m["text"],
		FriendlyTo: m["friendlyTo"], SpawnCount: m["spawnCount"], Zones: m["zones"], ZoneCount: m["zoneCount"],
	}
	for _, loc := range snapshot.AsList(m["locations"]) {
		t.Locations = append(t.Locations, readLocation(snapshot.AsMap(loc)))
	}
	return t
}

func readTargets(raw any) []Target {
	var out []Target
	for _, t := range snapshot.AsList(raw) {
		out = append(out, readTarget(t))
	}
	return out
}

func readEntry(raw any) Entry {
	m := snapshot.AsMap(raw)
	return Entry{
		ID: m["id"], Name: m["name"], Level: m["level"], RequiredLevel: m["requiredLevel"], Zone: m["zone"],
		ObjectivesText: m["objectivesText"], NextQuest: m["nextQuest"], InGuide: m["inGuide"],
		InLog:     m["inLog"] == true,
		StartedBy: readTargets(m["startedBy"]), FinishedBy: readTargets(m["finishedBy"]), Objectives: readTargets(m["objectives"]),
	}
}

func readEntries(raw any) []Entry {
	var out []Entry
	for _, e := range snapshot.AsList(raw) {
		out = append(out, readEntry(e))
	}
	return out
}

// readDatabase reads the questDb section and the answers to /mcp quest.
func readDatabase(s snapshot.Snapshot) Database {
	section := s.Map("questDb")
	_, has := s["questDb"]
	db := Database{Present: has, Quests: readEntries(section["quests"]), Unknown: snapshot.AsList(section["unknownQuestIDs"])}
	if source := snapshot.AsMap(section["source"]); len(source) > 0 {
		db.Source = source
	}
	for _, raw := range s.List("questLookups") {
		m := snapshot.AsMap(raw)
		saved := SavedLookup{Quests: readEntries(m["quests"]), NPCs: readTargets(m["npcs"])}
		saved.Query, _ = m["query"].(string)
		db.Lookups = append(db.Lookups, saved)
	}
	return db
}

func readGuide(s snapshot.Snapshot) GuideData {
	m := s.Map("guide")
	g := GuideData{Addon: m["addon"], NextGuide: m["nextGuide"], TextUnavailable: m["textUnavailable"] == true}
	g.Name, _ = m["name"].(string)
	g.Total, _ = snapshot.AsInt(m["totalSteps"])
	g.Done, _ = snapshot.AsInt(m["doneSteps"])
	g.Passed, _ = snapshot.AsInt(m["passedSteps"])
	for _, raw := range snapshot.AsList(m["next"]) {
		sm := snapshot.AsMap(raw)
		step := Step{Line: sm["line"], Optional: sm["optional"] == true, Passed: sm["passed"] == true}
		step.Text, _ = sm["text"].(string)
		for _, tagRaw := range snapshot.AsList(sm["quests"]) {
			tm := snapshot.AsMap(tagRaw)
			tag := Tag{Objective: tm["objective"]}
			tag.ID, _ = snapshot.AsInt(tm["id"])
			tag.Action, _ = tm["action"].(string)
			step.Tags = append(step.Tags, tag)
		}
		g.Next = append(g.Next, step)
	}
	return g
}
