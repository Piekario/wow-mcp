// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeons

import (
	"strings"

	"wowmcp/internal/snapshot"
)

func firstPositive(vals ...any) int64 {
	for _, v := range vals {
		if n, ok := snapshot.AsInt(v); ok && n > 0 {
			return n
		}
	}
	return 0
}

// readClientDungeons reads level ranges from client data (C_LFGList or GetLFGDungeonInfo).
func readClientDungeons(s snapshot.Snapshot) []Dungeon {
	d := s.Map("dungeons")
	var out []Dungeon
	seen := map[string]bool{}
	for _, raw := range snapshot.AsList(d["activities"]) {
		a := snapshot.AsMap(raw)
		name, _ := a["name"].(string)
		cat, _ := a["category"].(string)
		lc := strings.ToLower(cat)
		if name == "" || seen[name] || (cat != "" && !strings.Contains(lc, "dungeon") && !strings.Contains(lc, "raid")) {
			continue
		}
		seen[name] = true
		players := firstPositive(a["maxPlayers"])
		out = append(out, Dungeon{
			Name:    name,
			ID:      firstPositive(a["id"]),
			Min:     firstPositive(a["minLevelSuggestion"], a["minLevel"]),
			Max:     firstPositive(a["maxLevelSuggestion"], a["maxLevel"]),
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
		players := firstPositive(a["maxPlayers"])
		out = append(out, Dungeon{
			Name:    name,
			ID:      firstPositive(a["id"]),
			MapID:   firstPositive(a["mapID"]),
			Min:     firstPositive(a["minRecLevel"], a["recLevel"], a["minLevel"]),
			Max:     firstPositive(a["maxRecLevel"], a["maxLevel"]),
			Players: players,
			Raid:    players > 5,
		})
	}
	return out
}

func readLockouts(s snapshot.Snapshot) []Lockout {
	var out []Lockout
	for _, raw := range snapshot.AsList(s.Map("dungeons")["lockouts"]) {
		l := snapshot.AsMap(raw)
		lockout := Lockout{
			Name: l["name"], Difficulty: l["difficulty"], DifficultyID: l["difficultyID"],
			InstanceID: l["instanceID"], Raid: l["raid"], MaxPlayers: l["maxPlayers"],
			Released: l["locked"] == false,
		}
		lockout.InstanceMapID, _ = snapshot.AsInt(l["instanceID"])
		lockout.Killed, lockout.HasKilled = snapshot.AsInt(l["killed"])
		lockout.Encounters, _ = snapshot.AsInt(l["encounters"])
		lockout.ResetAt, lockout.HasReset = snapshot.AsInt(l["resetAt"])
		out = append(out, lockout)
	}
	return out
}
