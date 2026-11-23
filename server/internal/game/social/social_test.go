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
	out := View(s, Query{}, fixtures.At(2*time.Hour))
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "social", text, `"Tankadin"`, `"dungeon buddy"`, `"capturedAge"`, `"name":"Night Watch"`, `"onlineMembers"`, `"offlineHidden":1`, "privacy")
	g := out["guild"].(map[string]any)
	members := g["onlineMembers"].([]any)
	if len(members) != 2 || members[0].(map[string]any)["name"] != "Tankadin" {
		t.Errorf("online members sorted by rank: %v", members)
	}
	if strings.Contains(fixtures.JSON(t, g), "Oldfriend") {
		t.Error("offline hidden by default")
	}
	all := View(s, Query{IncludeOffline: true}, fixtures.RealTime())["guild"].(map[string]any)
	if len(all["members"].([]any)) != 3 || all["offlineHidden"] != nil {
		t.Errorf("includeOffline: %v", all)
	}
}

func TestViewMissingSectionsGiveHints(t *testing.T) {
	out := View(snapshot.Snapshot{}, Query{}, fixtures.RealTime())
	if out["friends"] != nil || out["guild"] != nil {
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
