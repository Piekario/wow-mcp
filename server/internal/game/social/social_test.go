// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestViewFriendsAndGuild(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := View(s, Query{}, fixtures.At(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "social", text, `"Tankadin"`, `"dungeon buddy"`, `"capturedAge"`, `"name":"Night Watch"`, `"onlineMembers"`, `"offlineHidden":1`, "privacy")
	g := out["guild"].(map[string]any)
	members := g["onlineMembers"].([]any)
	if len(members) != 2 || members[0].(map[string]any)["name"] != "Healbot" {
		t.Errorf("online members closest to the character's level first: %v", members)
	}
	if strings.Contains(fixtures.JSON(t, g), "Oldfriend") {
		t.Error("offline hidden by default")
	}
	view, _ := View(s, Query{IncludeOffline: true}, fixtures.RealTime())
	all := view["guild"].(map[string]any)
	if len(all["members"].([]any)) != 3 || all["offlineHidden"] != nil {
		t.Errorf("includeOffline: %v", all)
	}
}

func TestViewMissingSectionsGiveHints(t *testing.T) {
	out, _ := View(snapshot.Snapshot{}, Query{}, fixtures.RealTime())
	if out["friends"] != nil || out["guild"] != nil || out["groupCandidates"] != nil {
		t.Error("no data without sections")
	}
	fixtures.WantAll(t, "hints", fixtures.JSON(t, out), "friendsHint", "guildHint", "/mcp reload", "never asks the server")
}

func TestGuildOnlineOnlyNote(t *testing.T) {
	s := snapshot.Snapshot{"guild": map[string]any{"name": "Big", "onlineOnly": true, "members": []any{}}}
	if Guild(s, false, snapshot.Freshness{})["onlineOnlyNote"] == nil {
		t.Error("large guild: note")
	}
}

func TestPvP(t *testing.T) {
	if PvP(snapshot.Snapshot{}) != nil {
		t.Error("nil without a section")
	}
	p := PvP(fixtures.Modern(t))
	if p["honorableKills"] != int64(1234) || p["rankName"] != "Knight" {
		t.Errorf("pvp: %v", p)
	}
	// Forever (like a real client) has no classic PvP ranks.
	if p := PvP(fixtures.Forever(t)); p["honor"] != int64(340) || p["rank"] != nil {
		t.Errorf("pvp forever: %v", p)
	}
}

func TestRosterOrder(t *testing.T) {
	g := GuildState{Members: []Member{
		{Name: "bob", RankIndex: int64(1), Online: true},
		{Name: "Zed", RankIndex: int64(0), Online: true},
		{Name: "Amy", RankIndex: int64(1), Online: true},
		{Name: "Off", RankIndex: int64(0)},
	}}
	r := g.Roster(false)
	if r.OfflineHidden != 1 || len(r.Shown) != 3 || r.Shown[0].Name != "Zed" || r.Shown[1].Name != "Amy" {
		t.Errorf("roster: %+v", r)
	}
	if got := g.Roster(true); got.OfflineHidden != 0 || len(got.Shown) != 4 {
		t.Errorf("roster with offline: %+v", got)
	}
}

func TestInGuildWithoutField(t *testing.T) {
	if in, known := InGuild(snapshot.Snapshot{"guild": map[string]any{"members": []any{}}}); !in || !known {
		t.Error("a member list without inGuild means a guild")
	}
	if _, known := InGuild(snapshot.Snapshot{}); known {
		t.Error("no section, not known")
	}
}

func person(name, class string, level int64, online bool, source Source) Person {
	return Person{Name: name, ClassToken: class, Level: level, HasLevel: true, Online: online, Sources: []Source{source}}
}

func TestFindCandidatesSortsByRoleAndLevel(t *testing.T) {
	people := []Person{
		person("Tank", "WARRIOR", 22, true, FromGuild),
		person("Pally", "PALADIN", 20, true, FromGuild),
		person("Pally", "PALADIN", 20, true, FromFriends),
		person("Priest", "PRIEST", 25, true, FromFriends),
		person("Mage", "MAGE", 26, true, FromGuild),
		person("Away", "MAGE", 20, false, FromGuild),
		person("Me", "HUNTER", 20, true, FromGuild),
		{Name: "Odd", Class: "Paladyn", Level: 19, HasLevel: true, Online: true, Sources: []Source{FromGuild}},
		{Name: "NoLevel", ClassToken: "ROGUE", Online: true, Sources: []Source{FromGuild}},
	}
	c := FindCandidates(people, Self{Name: "Me-Realm", Level: 20, HasLevel: true}, 5)
	names := func(list []Person) []string {
		var out []string
		for _, p := range list {
			out = append(out, p.Name)
		}
		return out
	}
	if got := names(c.Tank); len(got) != 2 || got[0] != "Pally" || got[1] != "Tank" {
		t.Errorf("tank-capable: %v", got)
	}
	if got := names(c.Healer); len(got) != 2 || got[0] != "Pally" || got[1] != "Priest" {
		t.Errorf("healer-capable: %v", got)
	}
	if got := names(c.Damage); len(got) != 3 {
		t.Errorf("damage-capable: %v", got)
	}
	if got := names(c.Unknown); len(got) != 1 || got[0] != "Odd" {
		t.Errorf("unknown class: %v", got)
	}
	if c.Total != 4 || c.Offline != 1 || c.OnlineWithoutLevel != 1 || c.Low != 15 || c.High != 25 {
		t.Errorf("counts: %+v", c)
	}
	if len(c.Healer[0].Sources) != 2 {
		t.Errorf("a person on both lists appears once with both sources: %v", c.Healer[0])
	}
	if got := FindCandidates(people, Self{}, 5); !got.NoLevel || got.Total != 0 {
		t.Errorf("without a character level: %+v", got)
	}
}

func TestViewGroupCandidatesAndSummary(t *testing.T) {
	out, err := View(fixtures.Forever(t), Query{}, fixtures.At(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	gc := out["groupCandidates"].(map[string]any)
	fixtures.WantAll(t, "candidates", fixtures.JSON(t, gc), `"levels":"15-25"`, `"tankCapable"`, `"healerCapable"`, `"Tankadin"`, `"source":["friends","guild"]`)
	if strings.Contains(fixtures.JSON(t, gc), "Oldfriend") {
		t.Error("an offline player of level 60 is not a candidate")
	}
	summary := out["guild"].(map[string]any)["summary"].(map[string]any)
	fixtures.WantAll(t, "summary", fixtures.JSON(t, summary), `"listed":3`, `"online":2`, `"Paladin":1`, `"20-29":1`, `"10-19":1`, `"60-69":1`)
	if wide, _ := View(fixtures.Forever(t), Query{LevelRange: 1}, fixtures.At(time.Hour)); wide["groupCandidates"].(map[string]any)["count"] != 1 {
		t.Errorf("levelRange 1 keeps only Healbot (level 19) of a level 20 character: %v", wide["groupCandidates"])
	}
	if _, err := View(fixtures.Forever(t), Query{LevelRange: -1}, fixtures.At(time.Hour)); err == nil {
		t.Error("a negative levelRange is an error")
	}
}

func TestNoGuildStillGivesFriendCandidates(t *testing.T) {
	s := snapshot.Snapshot{
		"character": map[string]any{"name": "Me", "level": int64(20)},
		"guild":     map[string]any{"inGuild": false},
		"friends":   map[string]any{"friends": []any{map[string]any{"name": "Pal", "level": int64(21), "class": "Paladin", "classToken": "PALADIN", "connected": true}}},
	}
	out, err := View(s, Query{}, snapshot.Freshness{})
	if err != nil || out["guild"].(map[string]any)["inGuild"] != false {
		t.Fatalf("not in a guild is a valid answer: %v %v", out, err)
	}
	if gc := out["groupCandidates"].(map[string]any); gc["count"] != 1 || gc["tankCapable"] == nil {
		t.Errorf("friend candidates: %v", gc)
	}
}
