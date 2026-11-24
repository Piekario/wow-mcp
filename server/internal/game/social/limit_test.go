// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import (
	"fmt"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

// bigGuild is a synthetic guild of n members, most of them online, around the levels 20-35 of a level 25 character.
func bigGuild(n int) snapshot.Snapshot {
	classes := []string{"Warrior", "Paladin", "Hunter", "Rogue", "Priest", "Shaman", "Mage", "Warlock", "Druid"}
	members := make([]any, n)
	for i := range members {
		members[i] = map[string]any{
			"name": fmt.Sprintf("Member%03d", i), "rank": "Member", "rankIndex": int64(3), "level": int64(10 + i%40),
			"class": classes[i%len(classes)], "classToken": []string{"WARRIOR", "PALADIN", "HUNTER", "ROGUE", "PRIEST", "SHAMAN", "MAGE", "WARLOCK", "DRUID"}[i%len(classes)],
			"zone": "Thousand Needles", "note": "plays on weekends and evenings", "online": i%5 != 0,
		}
	}
	return snapshot.Snapshot{
		"character": map[string]any{"name": "Piekasz", "level": int64(25), "class": "Warlock"},
		"guild": map[string]any{"name": "Big Guild", "rank": "Member", "rankIndex": int64(3), "inGuild": true,
			"total": int64(n), "online": int64(n * 4 / 5), "members": members},
	}
}

func TestDefaultAnswerOfALargeGuildIsSmall(t *testing.T) {
	out, err := View(bigGuild(300), Query{}, snapshot.Freshness{})
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	if len(text) > 20*1024 {
		t.Fatalf("default answer is %d bytes, want under 20 KB", len(text))
	}
	t.Logf("default answer: %d bytes", len(text))
	guild := out["guild"].(map[string]any)
	list := guild["memberList"].(map[string]any)
	if len(guild["onlineMembers"].([]any)) != DefaultMemberLimit || list["shown"] != DefaultMemberLimit || list["total"] != 240 || list["notShown"] != 200 {
		t.Errorf("memberList = %v", list)
	}
	first := guild["onlineMembers"].([]any)[0].(map[string]any)
	if level := first["level"].(int64); level < 24 || level > 26 {
		t.Errorf("closest to the character's level first, got %v", first)
	}
}

func TestMemberLimitAndQuery(t *testing.T) {
	s := bigGuild(300)
	out, _ := View(s, Query{Limit: -1}, snapshot.Freshness{})
	if all := out["guild"].(map[string]any)["onlineMembers"].([]any); len(all) != 240 {
		t.Errorf("limit -1 lists every online member, got %d", len(all))
	}
	out, _ = View(s, Query{Limit: 5, Search: "priest"}, snapshot.Freshness{})
	list := out["guild"].(map[string]any)["memberList"].(map[string]any)
	if list["shown"] != 5 || list["query"] != "priest" || list["total"] != 27 || list["howToGetMore"] == nil {
		t.Errorf("memberList = %v", list)
	}
	if _, err := View(s, Query{Limit: -2}, snapshot.Freshness{}); err == nil {
		t.Error("limit below -1 is an error")
	}
}
