// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package group covers the party and raid: the composition at dump time (the group section) plus
// the gear of players the player inspected with the Inspect window (the inspect section, key = name).
package group

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/game/character"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_group.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Player    string `json:"player,omitempty" jsonschema:"player name: full gear from their latest Inspect"`
}

var roleNames = map[string]string{"TANK": "tank", "HEALER": "healer", "DAMAGER": "dps"}

// bigLevelGap: from this level difference on the group gets a warning.
const bigLevelGap = 6

// inspectSummary is a digest of the Inspect window data: average item level and age.
func inspectSummary(i map[string]any, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	if v, ok := i["averageItemLevel"]; ok {
		out["averageItemLevel"] = v
	}
	if at, ok := snapshot.AsInt(i["capturedAt"]); ok {
		out["inspectedAgo"] = snapshot.FormatAge(fresh.Age(at))
	}
	if g, ok := i["guild"]; ok {
		out["guild"] = g
	}
	return out
}

// View describes the group composition with warnings or, with the player parameter, the full Inspect of one player.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	inspected := s.Map("inspect")
	if q.Player != "" {
		return inspectedPlayer(inspected, q.Player, fresh)
	}

	out := map[string]any{}
	if party := s.Map("group"); party == nil {
		out["note"] = "The character was not in a group at dump time. Join a party or raid and type /mcp reload."
	} else {
		composition(party, inspected, out, fresh)
	}
	if ilvl, ok := character.AverageItemLevel(s); ok {
		out["myAverageItemLevel"] = ilvl
	}
	if len(inspected) > 0 {
		list := []any{}
		for _, name := range slices.Sorted(maps.Keys(inspected)) {
			i := snapshot.AsMap(inspected[name])
			e := map[string]any{"name": name, "level": i["level"], "class": i["class"]}
			snapshot.Merge(e, inspectSummary(i, fresh))
			list = append(list, e)
		}
		out["inspected"] = list
		out["inspectedNote"] = "A player's full gear: wow_group with the player parameter."
	}
	return out, nil
}

// inspectedPlayer returns a player's whole Inspect record (a name without the realm matches too).
func inspectedPlayer(inspected map[string]any, player string, fresh snapshot.Freshness) (map[string]any, error) {
	names := slices.Sorted(maps.Keys(inspected))
	for _, name := range names {
		if strings.EqualFold(name, player) || strings.EqualFold(strings.SplitN(name, "-", 2)[0], player) {
			out := map[string]any{"player": inspected[name]}
			snapshot.Merge(out, inspectSummary(snapshot.AsMap(inspected[name]), fresh))
			return out, nil
		}
	}
	return nil, fmt.Errorf("no Inspect data for %q. Inspected players: %s. In game right-click a player, choose Inspect, then /mcp reload",
		player, snapshot.OrEmpty(strings.Join(names, ", ")))
}

// composition describes group members, classes, roles and levels and collects warnings.
func composition(party, inspected map[string]any, out map[string]any, fresh snapshot.Freshness) {
	roles := map[string]int{}
	classes := map[string]int{}
	var minLevel, maxLevel, sumLevel int64
	var problems []string
	members := []any{}
	for _, raw := range snapshot.AsList(party["members"]) {
		m := snapshot.AsMap(raw)
		e := map[string]any{}
		for k, v := range m {
			e[k] = v
		}
		name, _ := m["name"].(string)
		if role, ok := roleNames[fmt.Sprint(m["role"])]; ok {
			e["role"] = role
			roles[role]++
		}
		if class, ok := m["class"].(string); ok {
			classes[class]++
		}
		level, _ := snapshot.AsInt(m["level"])
		sumLevel += level
		if minLevel == 0 || level < minLevel {
			minLevel = level
		}
		if level > maxLevel {
			maxLevel = level
		}
		problems = append(problems, memberProblems(name, m)...)
		if i := snapshot.AsMap(inspected[name]); i != nil {
			e["inspect"] = inspectSummary(i, fresh)
		}
		members = append(members, e)
	}
	sort.SliceStable(members, func(i, j int) bool {
		gi, _ := snapshot.AsInt(snapshot.AsMap(members[i])["subgroup"])
		gj, _ := snapshot.AsInt(snapshot.AsMap(members[j])["subgroup"])
		return gi < gj
	})
	out["type"] = party["type"]
	out["size"] = len(members)
	out["members"] = members
	out["classes"] = classes
	if len(roles) > 0 {
		out["roles"] = roles
	}
	if len(members) > 0 {
		out["levels"] = map[string]any{"min": minLevel, "max": maxLevel, "average": float64(sumLevel*10/int64(len(members))) / 10}
		if maxLevel-minLevel >= bigLevelGap {
			problems = append(problems, fmt.Sprintf("large level difference (%d-%d)", minLevel, maxLevel))
		}
	}
	if party["type"] == "party" && len(roles) > 0 {
		if roles["tank"] == 0 {
			problems = append(problems, "nobody has the tank role")
		}
		if roles["healer"] == 0 {
			problems = append(problems, "nobody has the healer role")
		}
	}
	if len(problems) > 0 {
		out["warnings"] = problems
	}
	for _, k := range []string{"lootMethod", "lootThreshold"} {
		if v, ok := party[k]; ok {
			out[k] = v
		}
	}
}

// memberProblems are warnings about one member: offline, dead, out of range.
func memberProblems(name string, m map[string]any) []string {
	var problems []string
	if m["online"] == false {
		problems = append(problems, name+" is offline")
	}
	if m["dead"] == true {
		problems = append(problems, name+" is dead")
	}
	if m["inRange"] == false {
		problems = append(problems, name+" is out of range")
	}
	return problems
}
