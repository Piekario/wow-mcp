// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package group

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestGroupView(t *testing.T) {
	fresh := fixtures.At(time.Hour)
	out, err := View(fixtures.Forever(t), Query{}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "raid", fixtures.JSON(t, out), `"type":"raid"`, `"size":3`, `"Paladin":2`, `"averageItemLevel":18`, `"inspected"`, "Night Watch", `"myAverageItemLevel"`)

	party, _ := View(fixtures.Modern(t), Query{}, fresh)
	fixtures.WantAll(t, "party", fixtures.JSON(t, party), `"type":"party"`, `"tank":1`, `"healer":1`, `"leader":true`, "Healbot is out of range")

	player, err := View(fixtures.Forever(t), Query{Player: "tankadin"}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "inspect", fixtures.JSON(t, player), "Curvewood Dagger", `"mainHand"`)
	if _, err := View(fixtures.Forever(t), Query{Player: "Nobody"}, fresh); err == nil || !strings.Contains(err.Error(), "Tankadin") {
		t.Errorf("error for an unknown player: %v", err)
	}
	solo, _ := View(snapshot.Snapshot{}, Query{}, fresh)
	if solo["note"] == nil {
		t.Error("solo without a note")
	}
}

func TestAnalyseFindsGroupProblems(t *testing.T) {
	member := func(name string, level int64, role string, extra map[string]any) any {
		m := map[string]any{"name": name, "level": level, "role": role, "class": "Mage", "subgroup": int64(1)}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	s := snapshot.Snapshot{"group": map[string]any{"type": "party", "members": []any{
		member("Dps", 10, "DAMAGER", map[string]any{"online": false}),
		member("Hi", 20, "DAMAGER", map[string]any{"dead": true, "inRange": false}),
	}}}
	c := Analyse(s, fixtures.At(0)).Composition
	var kinds []ProblemKind
	for _, p := range c.Problems {
		kinds = append(kinds, p.Kind)
	}
	want := []ProblemKind{MemberOffline, MemberDead, MemberOutOfRange, LevelGap, NoTank, NoHealer}
	if len(kinds) != len(want) {
		t.Fatalf("problems: %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("problems: %v, want %v", kinds, want)
		}
	}
	if c.Roles[Damage] != 2 || c.Levels.Min != 10 || c.Levels.Max != 20 || c.Levels.Average != 15 {
		t.Errorf("composition: %+v %+v", c.Roles, c.Levels)
	}
}
