// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import "wowmcp/internal/snapshot"

// questEntry keeps the title as the dump holds it.
type questEntry struct {
	ID    int64
	HasID bool
	Title any
}

func readQuestLog(s snapshot.Snapshot) []questEntry {
	var out []questEntry
	for _, raw := range s.List("quests") {
		q := snapshot.AsMap(raw)
		id, ok := snapshot.AsInt(q["id"])
		out = append(out, questEntry{ID: id, HasID: ok, Title: q["title"]})
	}
	return out
}

// readCompleted returns the completed quest IDs and whether the dump has the list at all.
func readCompleted(s snapshot.Snapshot) (map[int64]bool, bool) {
	list := s.List("completedQuests")
	out := map[int64]bool{}
	for _, raw := range list {
		if id, ok := snapshot.AsInt(raw); ok {
			out[id] = true
		}
	}
	return out, list != nil
}

type bagEntry struct {
	count, quality int64
}

func readBagCounts(s snapshot.Snapshot) map[string]*bagEntry {
	out := map[string]*bagEntry{}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			e := out[name]
			if e == nil {
				e = &bagEntry{}
				e.quality, _ = snapshot.AsInt(item["quality"])
				out[name] = e
			}
			e.count += snapshot.ItemCount(item)
		}
	}
	return out
}

type recentDeath struct {
	At    int64
	HasAt bool
	Zone  string
}

func readRecentDeaths(s snapshot.Snapshot) []recentDeath {
	var out []recentDeath
	for _, raw := range snapshot.AsList(s.Map("deaths")["recent"]) {
		d := snapshot.AsMap(raw)
		at, ok := snapshot.AsInt(d["at"])
		zone, _ := d["zone"].(string)
		out = append(out, recentDeath{At: at, HasAt: ok, Zone: zone})
	}
	return out
}

func readZone(s snapshot.Snapshot) string {
	zone, _ := s.Character()["zone"].(string)
	return zone
}

// readPlayed returns the play time from the dump: total and at the current level.
func readPlayed(s snapshot.Snapshot) (total, level int64, ok bool) {
	p := s.Map("played")
	total, ok1 := snapshot.AsInt(p["totalSeconds"])
	level, ok2 := snapshot.AsInt(p["levelSeconds"])
	return total, level, ok1 && ok2
}

// readHeading gives the character's name, level and class as the dump holds them.
func readHeading(s snapshot.Snapshot) (name, level, class any) {
	c := s.Character()
	return c["name"], c["level"], c["class"]
}
