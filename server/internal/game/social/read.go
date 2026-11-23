// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import "wowmcp/internal/snapshot"

// InGuild says whether the character is in a guild; known = false when the dump does not know (no guild section).
// A member list without the inGuild field (addon up to 0.7.0) also means a guild.
func InGuild(s snapshot.Snapshot) (in, known bool) {
	g := readGuild(s)
	return g.In, g.Saved
}

func readCount(m map[string]any, key string) Count {
	n, ok := snapshot.AsInt(m[key])
	return Count{N: n, Known: ok}
}

func readCapture(sec map[string]any) Capture {
	return Capture{Captured: snapshot.CapturedOf(sec), Age: readCount(sec, "capturedAt")}
}

func readFriends(s snapshot.Snapshot) FriendList {
	sec := s.Map("friends")
	if sec == nil {
		return FriendList{}
	}
	f := FriendList{Saved: true, Total: readCount(sec, "total"), Online: readCount(sec, "online"), Capture: readCapture(sec)}
	for _, raw := range snapshot.AsList(sec["friends"]) {
		m := snapshot.AsMap(raw)
		online, _ := m["connected"].(bool)
		f.List = append(f.List, Friend{Name: m["name"], Level: m["level"], Class: m["class"], Area: m["area"], Notes: m["notes"], Online: online})
	}
	return f
}

func readGuild(s snapshot.Snapshot) GuildState {
	sec := s.Map("guild")
	if sec == nil {
		return GuildState{}
	}
	g := GuildState{Saved: true, In: true, Capture: readCapture(sec),
		Name: sec["name"], Rank: sec["rank"], RankIndex: sec["rankIndex"],
		Total: readCount(sec, "total"), Online: readCount(sec, "online")}
	if v, ok := sec["inGuild"].(bool); ok {
		g.In = v
	}
	g.OnlineOnly, _ = sec["onlineOnly"].(bool)
	_, g.HasRoster = sec["members"]
	for _, raw := range snapshot.AsList(sec["members"]) {
		m := snapshot.AsMap(raw)
		online, _ := m["online"].(bool)
		g.Members = append(g.Members, Member{Name: m["name"], Rank: m["rank"], RankIndex: m["rankIndex"],
			Level: m["level"], Class: m["class"], Zone: m["zone"], Online: online})
	}
	return g
}

func readPvP(s snapshot.Snapshot) PvPRecord {
	sec := s.Map("pvp")
	if sec == nil {
		return PvPRecord{}
	}
	return PvPRecord{Saved: true, HonorableKills: sec["honorableKills"], DishonorableKills: sec["dishonorableKills"],
		HighestRank: sec["highestRank"], Honor: sec["honor"], HonorMax: sec["honorMax"], HonorLevel: sec["honorLevel"],
		Rank: sec["rank"], RankName: sec["rankName"]}
}
