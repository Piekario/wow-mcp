// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package dungeons covers dungeons and raids. First what the client itself reports (the dungeons section: the LFG tool with
// level ranges), and when the client reports nothing, a built-in table of dungeons from the
// classic version of the game. The table's ranges are approximate (different sources give
// them to within a few levels), and Forever may change them, so the
// response always says where they come from. Dungeons from the client carry numeric IDs (the activity or dungeon ID,
// the instance's map ID) and need no English table for their level ranges; the table stays English-only, and only
// adds zone and notes to dungeons whose English name it knows.
package dungeons

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

type dungeonInfo struct {
	Name     string
	Min, Max int64
	Zone     string
	Note     string
	Raid     bool
	Players  int64
	Aliases  []string
	ID       int64 // activity ID (C_LFGList) or dungeon ID (GetLFGDungeonInfo) from the client, 0 for the table
	MapID    int64 // the instance's map ID (lfgMapID), 0 when the client did not say
}

var classicDungeons = []dungeonInfo{
	{Name: "Ragefire Chasm", Min: 13, Max: 18, Zone: "Orgrimmar", Note: "entrance in the Horde capital"},
	{Name: "Wailing Caverns", Min: 17, Max: 24, Zone: "The Barrens"},
	{Name: "The Deadmines", Min: 17, Max: 26, Zone: "Westfall", Aliases: []string{"Deadmines"}},
	{Name: "Shadowfang Keep", Min: 22, Max: 30, Zone: "Silverpine Forest"},
	{Name: "Blackfathom Deeps", Min: 24, Max: 32, Zone: "Ashenvale"},
	{Name: "The Stockade", Min: 24, Max: 32, Zone: "Stormwind City", Note: "entrance in the Alliance capital", Aliases: []string{"Stormwind Stockade", "Stockade"}},
	{Name: "Gnomeregan", Min: 29, Max: 38, Zone: "Dun Morogh"},
	{Name: "Razorfen Kraul", Min: 29, Max: 38, Zone: "The Barrens"},
	{Name: "Scarlet Monastery: Graveyard", Min: 28, Max: 38, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Library", Min: 29, Max: 39, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Armory", Min: 32, Max: 42, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Cathedral", Min: 35, Max: 45, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Razorfen Downs", Min: 37, Max: 46, Zone: "The Barrens"},
	{Name: "Uldaman", Min: 41, Max: 51, Zone: "Badlands"},
	{Name: "Zul'Farrak", Min: 44, Max: 54, Zone: "Tanaris"},
	{Name: "Maraudon", Min: 46, Max: 55, Zone: "Desolace"},
	{Name: "Sunken Temple", Min: 50, Max: 56, Zone: "Swamp of Sorrows", Aliases: []string{"The Temple of Atal'Hakkar", "Temple of Atal'Hakkar"}},
	{Name: "Blackrock Depths", Min: 52, Max: 60, Zone: "Burning Steppes"},
	{Name: "Lower Blackrock Spire", Min: 55, Max: 60, Zone: "Burning Steppes", Aliases: []string{"Blackrock Spire"}},
	{Name: "Dire Maul", Min: 55, Max: 60, Zone: "Feralas"},
	{Name: "Scholomance", Min: 58, Max: 60, Zone: "Western Plaguelands"},
	{Name: "Stratholme", Min: 58, Max: 60, Zone: "Eastern Plaguelands"},
	{Name: "Upper Blackrock Spire", Min: 58, Max: 60, Zone: "Burning Steppes", Players: 10, Raid: true, Aliases: []string{"Blackrock Spire"}},
	{Name: "Zul'Gurub", Min: 60, Max: 60, Zone: "Stranglethorn Vale", Players: 20, Raid: true},
	{Name: "Ruins of Ahn'Qiraj", Min: 60, Max: 60, Zone: "Silithus", Players: 20, Raid: true},
	{Name: "Molten Core", Min: 60, Max: 60, Zone: "Burning Steppes", Players: 40, Raid: true},
	{Name: "Onyxia's Lair", Min: 60, Max: 60, Zone: "Dustwallow Marsh", Players: 40, Raid: true},
	{Name: "Blackwing Lair", Min: 60, Max: 60, Zone: "Burning Steppes", Players: 40, Raid: true},
	{Name: "Temple of Ahn'Qiraj", Min: 60, Max: 60, Zone: "Silithus", Players: 40, Raid: true},
	{Name: "Naxxramas", Min: 60, Max: 60, Zone: "Eastern Plaguelands", Players: 40, Raid: true},
}

// Query is the input of wow_dungeons.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	All       bool   `json:"all,omitempty" jsonschema:"true = all dungeons and raids, not just those matching the level"`
	Name      string `json:"name,omitempty" jsonschema:"fragment of a dungeon name: only that dungeon, with its quests"`
}

// gameDungeons reads level ranges from client data (C_LFGList or GetLFGDungeonInfo).
func gameDungeons(s snapshot.Snapshot) []dungeonInfo {
	d := s.Map("dungeons")
	var out []dungeonInfo
	seen := map[string]bool{}
	pick := func(vals ...any) int64 {
		for _, v := range vals {
			if n, ok := snapshot.AsInt(v); ok && n > 0 {
				return n
			}
		}
		return 0
	}
	for _, raw := range snapshot.AsList(d["activities"]) {
		a := snapshot.AsMap(raw)
		name, _ := a["name"].(string)
		cat, _ := a["category"].(string)
		lc := strings.ToLower(cat)
		if name == "" || seen[name] || (cat != "" && !strings.Contains(lc, "dungeon") && !strings.Contains(lc, "raid")) {
			continue
		}
		seen[name] = true
		players := pick(a["maxPlayers"])
		out = append(out, dungeonInfo{
			Name:    name,
			ID:      pick(a["id"]),
			Min:     pick(a["minLevelSuggestion"], a["minLevel"]),
			Max:     pick(a["maxLevelSuggestion"], a["maxLevel"]),
			Players: players,
			Raid:    strings.Contains(lc, "raid") || players > 5,
		})
	}
	for _, raw := range snapshot.AsList(d["lfg"]) {
		a := snapshot.AsMap(raw)
		name, _ := a["name"].(string)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		players := pick(a["maxPlayers"])
		out = append(out, dungeonInfo{
			Name:    name,
			ID:      pick(a["id"]),
			MapID:   pick(a["mapID"]),
			Min:     pick(a["minRecLevel"], a["recLevel"], a["minLevel"]),
			Max:     pick(a["maxRecLevel"], a["maxLevel"]),
			Players: players,
			Raid:    players > 5,
		})
	}
	// Zone, notes and other names from the table, if the dungeon is in it.
	for i := range out {
		if ref, ok := findClassicDungeon(out[i].Name); ok {
			out[i].Zone, out[i].Note, out[i].Aliases = ref.Zone, ref.Note, ref.Aliases
		}
	}
	return out
}

func findClassicDungeon(name string) (dungeonInfo, bool) {
	n := strings.ToLower(strings.TrimPrefix(name, "The "))
	for _, d := range classicDungeons {
		if strings.ToLower(strings.TrimPrefix(d.Name, "The ")) == n {
			return d, true
		}
	}
	return dungeonInfo{}, false
}

// dungeonQuests returns the quest log quests tied to a dungeon: the log header
// is the dungeon name, or the dungeon name appears in the quest title or objectives.
func dungeonQuests(s snapshot.Snapshot, d dungeonInfo, playerLevel, green int64) []any {
	names := []string{strings.ToLower(d.Name), strings.ToLower(strings.TrimPrefix(d.Name, "The "))}
	for _, a := range d.Aliases {
		names = append(names, strings.ToLower(a))
	}
	mentions := func(text string) bool {
		t := strings.ToLower(text)
		for _, n := range names {
			if n != "" && strings.Contains(t, n) {
				return true
			}
		}
		return false
	}
	out := []any{}
	for _, raw := range s.List("quests") {
		q := snapshot.AsMap(raw)
		zone, _ := q["zone"].(string)
		title, _ := q["title"].(string)
		goals := quests.Objectives(q)
		if !mentions(zone) && !mentions(title) && !mentions(strings.Join(goals, " ")) {
			continue
		}
		e := map[string]any{"title": title, "id": q["id"], "level": q["level"]}
		if lvl, ok := snapshot.AsInt(q["level"]); ok && playerLevel > 0 {
			e["difficulty"] = quests.Difficulty(lvl, playerLevel, green)
		}
		if q["complete"] == true {
			e["complete"] = true
		} else if len(goals) > 0 {
			e["objectives"] = strings.Join(goals, "; ")
		}
		out = append(out, e)
	}
	return out
}

// View splits dungeons into those matching the level (fitNow), those available in a few levels
// (soon) and the rest, with quest log quests and instance lockouts.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	level, _ := s.LevelAndMoney()
	// Unlike in the quest plan: a value from the game <= 0 does not switch to the table.
	green, ok := snapshot.AsInt(s.Character()["questGreenRange"])
	if !ok {
		green = quests.GreenRange(level)
	}
	list := gameDungeons(s)
	source := "game"
	sourceNote := "The level ranges come from the game client (the group finder tool)."
	if len(list) == 0 {
		list = classicDungeons
		source = "classic"
		sourceNote = "The client did not report a dungeon list, so the ranges come from the classic game table and are approximate; Forever may change them. If the game has a group finder tool, open it once and type /mcp reload."
	}

	lockedInstances := lockedInstanceIDs(s)
	var fit, soon, other []any
	nq := strings.ToLower(q.Name)
	for _, d := range list {
		if nq != "" && !strings.Contains(strings.ToLower(d.Name), nq) {
			continue
		}
		e := map[string]any{"name": d.Name}
		if d.ID > 0 {
			e["id"] = d.ID
		}
		if d.MapID > 0 {
			e["mapID"] = d.MapID
			if lockedInstances[d.MapID] {
				e["locked"] = true
			}
		}
		if d.Min > 0 {
			e["levels"] = fmt.Sprintf("%d-%d", d.Min, d.Max)
		}
		if d.Zone != "" {
			e["zone"] = d.Zone
		}
		if d.Note != "" {
			e["note"] = d.Note
		}
		if d.Raid {
			e["raid"] = true
		}
		if d.Players > 0 {
			e["players"] = d.Players
		}
		related := dungeonQuests(s, d, level, green)
		if len(related) > 0 {
			e["quests"] = related
		}
		switch {
		case nq != "":
			fit = append(fit, e)
		case d.Min > 0 && level >= d.Min-1 && (d.Max == 0 || level <= d.Max):
			fit = append(fit, e)
		case d.Min > 0 && level < d.Min-1 && level >= d.Min-6:
			e["levelsToGo"] = d.Min - 1 - level
			soon = append(soon, e)
		case q.All || len(related) > 0:
			other = append(other, e)
		}
	}
	out := map[string]any{
		"level":      level,
		"source":     source,
		"sourceNote": sourceNote,
		"fitNow":     orEmpty(fit),
	}
	if len(soon) > 0 {
		out["soon"] = soon
	}
	if len(other) > 0 {
		out["other"] = other
		if !q.All {
			out["otherNote"] = "Dungeons outside your level range that you have quests for in the log."
		}
	}
	if lockouts := lockoutList(s, fresh); len(lockouts) > 0 {
		out["lockouts"] = lockouts
	}
	if source == "classic" {
		s.AddEnglishOnlyNote(out, "The dungeon list, zones, notes and the names used to find dungeon quests in the quest log come from a table of English dungeon names, because the client gave no dungeon list")
	} else if n := withoutTableData(list); n > 0 {
		s.AddEnglishOnlyNote(out, fmt.Sprintf("Levels come from the client, but the zone and notes of a dungeon come from a table of English dungeon names, so they are missing for %d dungeons", n))
	}
	return out
}

// withoutTableData counts the client's dungeons the English table did not enrich (no zone from it).
func withoutTableData(list []dungeonInfo) int {
	n := 0
	for _, d := range list {
		if d.Zone == "" {
			n++
		}
	}
	return n
}

// lockedInstanceIDs is the set of instance map IDs the character is saved to (the lockouts' instanceID, addon 0.10.0).
func lockedInstanceIDs(s snapshot.Snapshot) map[int64]bool {
	ids := map[int64]bool{}
	for _, raw := range snapshot.AsList(s.Map("dungeons")["lockouts"]) {
		l := snapshot.AsMap(raw)
		if id, ok := snapshot.AsInt(l["instanceID"]); ok && id > 0 && l["locked"] != false {
			ids[id] = true
		}
	}
	return ids
}

// orEmpty turns nil into an empty list so JSON has [] instead of null.
func orEmpty(list []any) []any {
	if list == nil {
		return []any{}
	}
	return list
}

// lockoutList describes the saved instance lockouts with the time to reset.
func lockoutList(s snapshot.Snapshot, fresh snapshot.Freshness) []any {
	out := []any{}
	for _, raw := range snapshot.AsList(s.Map("dungeons")["lockouts"]) {
		l := snapshot.AsMap(raw)
		e := map[string]any{"name": l["name"]}
		for _, k := range []string{"difficulty", "difficultyID", "instanceID", "raid", "maxPlayers"} {
			if v, ok := l[k]; ok {
				e[k] = v
			}
		}
		if killed, ok := snapshot.AsInt(l["killed"]); ok {
			total, _ := snapshot.AsInt(l["encounters"])
			e["progress"] = fmt.Sprintf("%d/%d", killed, total)
		}
		if at, ok := snapshot.AsInt(l["resetAt"]); ok {
			if left, passed := fresh.Until(at); passed {
				e["reset"] = "already reset"
			} else {
				e["resetIn"] = snapshot.FormatAge(left)
			}
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(snapshot.AsMap(out[i])["name"]) < fmt.Sprint(snapshot.AsMap(out[j])["name"])
	})
	return out
}
