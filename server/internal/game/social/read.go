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
		token, _ := m["classToken"].(string)
		f.List = append(f.List, Friend{Name: m["name"], Level: m["level"], Class: m["class"], Area: m["area"], Notes: m["notes"],
			ClassToken: token, Online: online})
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
		member := Member{Name: m["name"], Rank: m["rank"], RankIndex: m["rankIndex"],
			Level: m["level"], Class: m["class"], Zone: m["zone"], Online: online}
		member.Note, _ = m["note"].(string)
		member.ClassToken, _ = m["classToken"].(string)
		member.OfflineDays, member.HasOfflineDays = snapshot.AsInt(m["offlineDays"])
		g.Members = append(g.Members, member)
	}
	return g
}

// HonorStats is the numbers of the pvp section the history records; Saved is false without the section.
type HonorStats struct {
	Saved                 bool
	Honor, HonorableKills int64
	HasHonor, HasKills    bool
	HonorLevel            int64
	HasHonorLevel         bool
}

// Honor reads the honor and honorable kills of a dump.
func Honor(s snapshot.Snapshot) HonorStats {
	sec := s.Map("pvp")
	if sec == nil {
		return HonorStats{}
	}
	h := HonorStats{Saved: true}
	h.Honor, h.HasHonor = snapshot.AsInt(sec["honor"])
	h.HonorableKills, h.HasKills = snapshot.AsInt(sec["honorableKills"])
	h.HonorLevel, h.HasHonorLevel = snapshot.AsInt(sec["honorLevel"])
	return h
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

// Source says where a Person comes from.
type Source string

const (
	FromGuild   Source = "guild"
	FromFriends Source = "friends"
)

// Person is a guild member or a friend as the group-finding rules see them.
type Person struct {
	Name, Class, ClassToken, Zone, Rank, Note string
	Level                                     int64
	HasLevel, Online                          bool
	Sources                                   []Source
}

// Self is the character the answer is for.
type Self struct {
	Name     string
	Level    int64
	HasLevel bool
}

func readSelf(s snapshot.Snapshot) Self {
	c := s.Map("character")
	if c == nil {
		return Self{}
	}
	var self Self
	self.Name, _ = c["name"].(string)
	self.Level, self.HasLevel = snapshot.AsInt(c["level"])
	return self
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func personOf(name, class, token, zone, rank, note string, level any, online bool, source Source) Person {
	p := Person{Name: name, Class: class, ClassToken: token, Zone: zone, Rank: rank, Note: note, Online: online, Sources: []Source{source}}
	p.Level, p.HasLevel = snapshot.AsInt(level)
	return p
}

func (g GuildState) people() []Person {
	var out []Person
	for _, m := range g.Members {
		out = append(out, personOf(text(m.Name), text(m.Class), m.ClassToken, text(m.Zone), text(m.Rank), m.Note, m.Level, m.Online, FromGuild))
	}
	return out
}

func (f FriendList) people() []Person {
	var out []Person
	for _, fr := range f.List {
		out = append(out, personOf(text(fr.Name), text(fr.Class), fr.ClassToken, text(fr.Area), "", text(fr.Notes), fr.Level, fr.Online, FromFriends))
	}
	return out
}
