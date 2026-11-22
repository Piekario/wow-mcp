// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package questguide answers "where do I pick up, do and turn in this quest" and "what is my next guide step".
//
// The server holds no quest data of its own. Everything comes from the dump: the questDb section (where quests
// start and end, read by the addon from the quest database the player has installed, QuestieDB), the
// questLookups section (answers to /mcp quest in game) and the guide section (the next steps of the player's
// Guidelime guide, e.g. TUGs). The quest text in the game is often out of date and World of Warcraft: Forever
// differs from Classic, so when the dump has no data the answer says how to get it instead of guessing.
package questguide

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_quest_lookup.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = most recently dumped character"`
	Quest     string `json:"quest,omitempty" jsonschema:"quest ID, or a fragment of a quest or NPC name; empty = every quest in the log"`
}

const (
	noDatabaseHint = "The dump has no quest database section (questDb). It needs addon 0.14.0 or newer and the Questie addon (with QuestieDB) installed in the game; then run /mcp reload. " +
		"Do NOT answer where a quest starts or ends from memory or from the quest text: World of Warcraft: Forever differs from Classic and the quest text is often out of date."
	notInDumpHint = "Not in the dump: it holds only the quests in the log, the quests of the next guide steps and the answers to /mcp quest. " +
		"Ask the player to type /mcp quest <exact quest name, NPC name or ID> in game and then /mcp reload. Do not guess the location from memory."
	sourceNote = "Locations come from the quest database installed in the player's game (the same data Questie shows on the map), not from the quest text. " +
		"Coordinates are map percentages (x from the left, y from the top) in the named zone."
)

// Quest states in an answer.
const (
	statusReady     = "in log, ready to turn in"
	statusActive    = "in log, in progress"
	statusFailed    = "in log, failed"
	statusCompleted = "completed"
	statusNotInLog  = "not in log"
)

// player is what the dump says about where the character stands.
type player struct {
	zone string
	x, y float64
	has  bool
}

func playerOf(s snapshot.Snapshot) player {
	c := s.Character()
	p := player{}
	p.zone, _ = c["zone"].(string)
	x, okX := snapshot.AsFloat(c["x"])
	y, okY := snapshot.AsFloat(c["y"])
	p.x, p.y, p.has = x, y, okX && okY && p.zone != ""
	return p
}

// compass names the direction of a map offset (y grows to the south).
func compass(dx, dy float64) string {
	names := []string{"east", "south-east", "south", "south-west", "west", "north-west", "north", "north-east"}
	angle := math.Atan2(dy, dx) // 0 = east, positive = clockwise on the map
	index := int(math.Round(angle/(math.Pi/4))+8) % 8
	return names[index]
}

// place turns one target from the dump (an NPC, object or item with sample locations) into an answer entry.
func place(raw any, p player) map[string]any {
	t := snapshot.AsMap(raw)
	out := map[string]any{}
	for _, key := range []string{"kind", "id", "name", "title", "text", "friendlyTo", "spawnCount", "zones", "zoneCount"} {
		if v, ok := t[key]; ok {
			out[key] = v
		}
	}
	locations := snapshot.AsList(t["locations"])
	if len(locations) == 0 {
		if t["kind"] == "item" {
			out["note"] = "an item: the dump does not say where it drops"
		} else if _, named := t["name"]; !named {
			out["note"] = "not in the quest database"
		}
		return out
	}
	var where []any
	for _, rawLoc := range locations {
		loc := snapshot.AsMap(rawLoc)
		entry := map[string]any{}
		zone, _ := loc["zone"].(string)
		if zone != "" {
			entry["zone"] = zone
		} else if id, ok := snapshot.AsInt(loc["areaID"]); ok {
			entry["zone"] = fmt.Sprintf("area %d", id)
		}
		if loc["inInstance"] == true {
			entry["inInstance"] = true
		}
		x, okX := snapshot.AsFloat(loc["x"])
		y, okY := snapshot.AsFloat(loc["y"])
		if okX && okY {
			entry["x"], entry["y"] = x, y
			if p.has && zone == p.zone {
				dx, dy := x-p.x, y-p.y
				distance := snapshot.Round1(math.Hypot(dx, dy))
				if distance < 1 {
					entry["fromPlayer"] = "right here"
				} else {
					entry["fromPlayer"] = fmt.Sprintf("%g map units %s of the character", distance, compass(dx, dy))
				}
			}
		}
		where = append(where, entry)
	}
	out["where"] = where
	if count, _ := snapshot.AsInt(t["spawnCount"]); count > int64(len(where)) {
		out["note"] = fmt.Sprintf("%d spawn points in total; %d shown", count, len(where))
	}
	return out
}

func places(raw any, p player) []any {
	var out []any
	for _, t := range snapshot.AsList(raw) {
		out = append(out, place(t, p))
	}
	return out
}

// describe is "Falla Sagewind (The Barrens 48.2, 32.8)" for the first target of a list, or "".
func describe(raw any) string {
	targets := snapshot.AsList(raw)
	if len(targets) == 0 {
		return ""
	}
	t := snapshot.AsMap(targets[0])
	name, _ := t["name"].(string)
	if name == "" {
		name = fmt.Sprintf("%v %v", t["kind"], t["id"])
	}
	locations := snapshot.AsList(t["locations"])
	if len(locations) == 0 {
		return name
	}
	loc := snapshot.AsMap(locations[0])
	zone, _ := loc["zone"].(string)
	if zone == "" {
		zone = "unknown zone"
	}
	x, okX := snapshot.AsFloat(loc["x"])
	y, okY := snapshot.AsFloat(loc["y"])
	switch {
	case loc["inInstance"] == true:
		return fmt.Sprintf("%s (inside %s)", name, zone)
	case okX && okY:
		return fmt.Sprintf("%s (%s %g, %g)", name, zone, x, y)
	}
	return fmt.Sprintf("%s (%s)", name, zone)
}

// logStates maps quest ID -> state for the quests in the log and the completed ones.
func logStates(s snapshot.Snapshot) map[int64]string {
	states := map[int64]string{}
	for _, raw := range snapshot.AsList(s["completedQuests"]) {
		if id, ok := snapshot.AsInt(raw); ok {
			states[id] = statusCompleted
		}
	}
	for _, raw := range s.List("quests") {
		q := snapshot.AsMap(raw)
		id, ok := snapshot.AsInt(q["id"])
		if !ok {
			continue
		}
		switch {
		case q["failed"] == true:
			states[id] = statusFailed
		case q["complete"] == true:
			states[id] = statusReady
		default:
			states[id] = statusActive
		}
	}
	return states
}

func statusOf(states map[int64]string, id int64) string {
	if state, ok := states[id]; ok {
		return state
	}
	return statusNotInLog
}

// nextStep is the one-line answer for a quest in the given state.
func nextStep(q map[string]any, status string) string {
	switch status {
	case statusReady:
		if where := describe(q["finishedBy"]); where != "" {
			return "Turn in at " + where
		}
		return "Ready to turn in, but the quest database does not know where"
	case statusActive:
		if where := describe(q["finishedBy"]); where != "" {
			return "Finish the objectives, then turn in at " + where
		}
	case statusNotInLog:
		if where := describe(q["startedBy"]); where != "" {
			return "Pick up from " + where
		}
	case statusFailed:
		return "Failed: abandon it and pick it up again"
	}
	return ""
}

// questView is one quest of the answer.
func questView(raw any, states map[int64]string, p player) map[string]any {
	q := snapshot.AsMap(raw)
	out := map[string]any{}
	for _, key := range []string{"id", "name", "level", "requiredLevel", "zone", "objectivesText", "nextQuest", "inGuide"} {
		if v, ok := q[key]; ok {
			out[key] = v
		}
	}
	id, _ := snapshot.AsInt(q["id"])
	status := statusOf(states, id)
	out["status"] = status
	if step := nextStep(q, status); step != "" {
		out["nextStep"] = step
	}
	if status != statusCompleted {
		if list := places(q["finishedBy"], p); list != nil {
			out["turnIn"] = list
		}
		if list := places(q["objectives"], p); list != nil && status != statusReady {
			out["objectives"] = list
		}
	}
	if status == statusNotInLog || status == statusFailed {
		if list := places(q["startedBy"], p); list != nil {
			out["pickUp"] = list
		}
	}
	return out
}

// matches reports whether a quest or NPC entry fits the query (an ID, or a name fragment in any letter case).
func matches(entry map[string]any, query string) bool {
	if query == "" {
		return true
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		got, ok := snapshot.AsInt(entry["id"])
		return ok && got == id
	}
	name, _ := entry["name"].(string)
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

// Lookup is the wow_quest_lookup answer: where the matching quests start, what they need and where they end.
func Lookup(s snapshot.Snapshot, q Query) map[string]any {
	query := strings.TrimSpace(q.Quest)
	out := map[string]any{}
	section := s.Map("questDb")
	lookups := s.List("questLookups")
	if _, has := s["questDb"]; !has && len(lookups) == 0 {
		out["available"] = false
		out["hint"] = noDatabaseHint
		return out
	}
	p := playerOf(s)
	states := logStates(s)
	if source := snapshot.AsMap(section["source"]); len(source) > 0 {
		out["source"] = source
	}
	if p.has {
		out["character"] = map[string]any{"zone": p.zone, "x": p.x, "y": p.y}
	}

	seen := map[int64]bool{}
	var quests, npcs []any
	addQuest := func(raw any, requireLog bool) {
		entry := snapshot.AsMap(raw)
		id, ok := snapshot.AsInt(entry["id"])
		if !ok || seen[id] || !matches(entry, query) {
			return
		}
		if requireLog && entry["inLog"] != true {
			return
		}
		seen[id] = true
		quests = append(quests, questView(raw, states, p))
	}
	// Without a query: the quest log. With one: also the guide quests and the answers to /mcp quest.
	for _, raw := range snapshot.AsList(section["quests"]) {
		addQuest(raw, query == "")
	}
	if query != "" {
		seenNpc := map[int64]bool{}
		for _, rawLookup := range lookups {
			lookup := snapshot.AsMap(rawLookup)
			asked, _ := lookup["query"].(string)
			sameQuestion := strings.EqualFold(strings.TrimSpace(asked), query)
			for _, raw := range snapshot.AsList(lookup["quests"]) {
				if sameQuestion {
					entry := snapshot.AsMap(raw)
					if id, ok := snapshot.AsInt(entry["id"]); ok && !seen[id] {
						seen[id] = true
						quests = append(quests, questView(raw, states, p))
					}
					continue
				}
				addQuest(raw, false)
			}
			for _, raw := range snapshot.AsList(lookup["npcs"]) {
				entry := snapshot.AsMap(raw)
				id, ok := snapshot.AsInt(entry["id"])
				if !ok || seenNpc[id] || !(sameQuestion || matches(entry, query)) {
					continue
				}
				seenNpc[id] = true
				npcs = append(npcs, place(raw, p))
			}
		}
	}

	out["count"] = len(quests)
	if quests != nil {
		out["quests"] = quests
	}
	if npcs != nil {
		out["npcs"] = npcs
	}
	out["note"] = sourceNote
	switch {
	case len(quests) == 0 && len(npcs) == 0 && query != "":
		out["hint"] = notInDumpHint
	case len(quests) == 0 && query == "":
		out["hint"] = "No quest of the log is in the quest database. For another quest pass quest=<name or ID>."
	}
	if unknown := snapshot.AsList(section["unknownQuestIDs"]); len(unknown) > 0 && query == "" {
		out["unknownQuestIDs"] = unknown
	}
	return out
}
