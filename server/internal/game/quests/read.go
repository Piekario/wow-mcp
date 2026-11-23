// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package quests

import "wowmcp/internal/snapshot"

// ReadLog turns the quests section and the character facts the quest rules need into types. An entry that is not
// a table stays in the list as an empty Quest, so counts match the dump.
func ReadLog(s snapshot.Snapshot) Log {
	_, present := s["quests"]
	log := Log{Present: present}
	log.Level, _ = s.LevelAndMoney()
	c := s.Character()
	log.GameGreen, log.HasGameGreen = snapshot.AsInt(c["questGreenRange"])
	log.MaxQuests, log.HasMaxQuests = snapshot.AsInt(c["maxQuests"])
	for _, raw := range s.List("quests") {
		log.Quests = append(log.Quests, readQuest(snapshot.AsMap(raw)))
	}
	return log
}

func readQuest(m map[string]any) Quest {
	q := Quest{ID: m["id"], Complete: m["complete"] == true, Failed: m["failed"] == true}
	q.Title, _ = m["title"].(string)
	q.Zone, _ = m["zone"].(string)
	q.Level, q.HasLevel = snapshot.AsInt(m["level"])
	q.ZoneMapID, _ = snapshot.AsInt(m["zoneMapID"])
	q.SuggestedGroup, _ = snapshot.AsInt(m["suggestedGroup"])
	for _, raw := range snapshot.AsList(m["objectives"]) {
		o := snapshot.AsMap(raw)
		obj := Objective{Done: o["done"] == true}
		obj.Text, obj.HasText = o["text"].(string)
		var haveOK, needOK bool
		obj.Have, haveOK = snapshot.AsInt(o["have"])
		obj.Need, needOK = snapshot.AsInt(o["need"])
		obj.HasCount = haveOK && needOK
		q.Objectives = append(q.Objectives, obj)
	}
	return q
}

// Objectives returns the objective texts of one quest table from the dump.
func Objectives(q map[string]any) []string {
	return readQuest(q).ObjectiveTexts()
}
