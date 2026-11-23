// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package group

import (
	"fmt"
	"maps"
	"slices"

	"wowmcp/internal/snapshot"
)

var roleOf = map[string]Role{"TANK": Tank, "HEALER": Healer, "DAMAGER": Damage}

func readParty(s snapshot.Snapshot) (Party, bool) {
	m := s.Map("group")
	if m == nil {
		return Party{}, false
	}
	p := Party{Type: m["type"]}
	p.LootMethod, p.HasLootMethod = m["lootMethod"]
	p.LootThreshold, p.HasLootThreshold = m["lootThreshold"]
	for _, raw := range snapshot.AsList(m["members"]) {
		p.Members = append(p.Members, readMember(raw))
	}
	return p, true
}

func readMember(raw any) Member {
	m := snapshot.AsMap(raw)
	member := Member{Raw: raw, Role: roleOf[fmt.Sprint(m["role"])]}
	member.Name, _ = m["name"].(string)
	member.Class, member.HasClass = m["class"].(string)
	member.Level, _ = snapshot.AsInt(m["level"])
	member.Subgroup, _ = snapshot.AsInt(m["subgroup"])
	member.Offline = m["online"] == false
	member.Dead = m["dead"] == true
	member.OutOfRange = m["inRange"] == false
	return member
}

// readInspected returns the Inspect records ordered by player name.
func readInspected(s snapshot.Snapshot) []Inspected {
	all := s.Map("inspect")
	var out []Inspected
	for _, name := range slices.Sorted(maps.Keys(all)) {
		m := snapshot.AsMap(all[name])
		i := Inspected{Name: name, Level: m["level"], Class: m["class"], Raw: all[name], Record: m != nil}
		i.AverageItemLevel, i.HasAverage = m["averageItemLevel"]
		i.CapturedAt, i.HasCapturedAt = snapshot.AsInt(m["capturedAt"])
		i.Guild, i.HasGuild = m["guild"]
		out = append(out, i)
	}
	return out
}
