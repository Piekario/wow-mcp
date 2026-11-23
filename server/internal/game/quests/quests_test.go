// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package quests

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestQuestDifficulty(t *testing.T) {
	cases := []struct {
		quest, player, green int64
		want                 string
	}{
		{25, 20, 9, "red"}, {23, 20, 9, "orange"}, {22, 20, 9, "yellow"}, {18, 20, 9, "yellow"},
		{17, 20, 9, "green"}, {11, 20, 9, "green"}, {10, 20, 9, "gray"},
	}
	for _, c := range cases {
		if got := Difficulty(c.quest, c.player, c.green); got != c.want {
			t.Errorf("Difficulty(%d, %d, %d) = %s, want %s", c.quest, c.player, c.green, got, c.want)
		}
	}
	if GreenRange(20) != 11 || GreenRange(60) != 17 {
		t.Error("GreenRange table")
	}
}

func TestQuestPlan(t *testing.T) {
	s := fixtures.Forever(t)
	quests := snapshot.AsList(s["quests"])
	quests = append(quests,
		map[string]any{"id": int64(1), "title": "Too Hard", "level": int64(26), "zone": "Ashenvale"},
		map[string]any{"id": int64(2), "title": "Old News", "level": int64(8), "zone": "Darkshore"},
		map[string]any{"id": int64(3), "title": "Done Deal", "level": int64(19), "zone": "Darkshore", "complete": true},
	)
	s["quests"] = quests
	out := Plan(s, PlanQuery{})
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "quest plan", text,
		`"difficulty":"red"`, `"difficulty":"gray"`, `Too hard (red): Too Hard [26]`,
		`Gray, almost no XP: Old News [8]`, `"readyToTurnIn":["Done Deal [19] (Darkshore)"]`,
		`"log":"5/20"`, `quests of level 10 and below are gray`, `"progressPercent":83`, // Cave Mushrooms: 5/5 + 0/1
	)
	zones := snapshot.AsList(out["zones"])
	if snapshot.AsMap(zones[0])["zone"] != "Darkshore" {
		t.Errorf("Darkshore should be first: %v", zones[0])
	}
	if only := Plan(s, PlanQuery{Zone: "ashen"}); only["count"] != 1 {
		t.Errorf("zone filter: %v", only["count"])
	}
}

func TestAnalyseRatesQuestsOnTheTypedResult(t *testing.T) {
	s := snapshot.Snapshot{
		"character": map[string]any{"level": int64(20)},
		"quests": []any{
			map[string]any{"title": "Hard", "level": int64(26), "zone": "A"},
			map[string]any{"title": "Done", "level": int64(20), "zone": "A", "complete": true},
			map[string]any{"title": "Old", "level": int64(5), "zone": "B", "failed": true},
		},
	}
	r := Analyse(s, PlanQuery{})
	if r.Count != 3 || r.Green != GreenRange(20) || r.GreenFromGame {
		t.Fatalf("result: %+v", r)
	}
	if len(r.TooHard) != 1 || r.TooHard[0].Quest.Title != "Hard" || len(r.Failed) != 1 || len(r.Ready) != 1 {
		t.Errorf("flags: %+v", r)
	}
	if r.Zones[0].Name != "A" || r.Zones[0].Ready != 1 || r.Zones[0].Quests[0].Status != ReadyToTurnIn && r.Zones[0].Quests[1].Status != ReadyToTurnIn {
		t.Errorf("zones: %+v", r.Zones)
	}
	if p, _ := (Quest{Complete: true}).Progress(); p != 100 {
		t.Errorf("a complete quest without objectives is 100%%, got %d", p)
	}
}
