// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package legacy

import (
	"strings"
	"testing"

	"wowmcp/internal/snapshot"
)

func node(name, tree string, rank, maxRank int64, canPurchase bool) map[string]any {
	return map[string]any{"name": name, "tree": tree, "rank": rank, "maxRank": maxRank, "canPurchase": canPurchase, "row": int64(1), "column": int64(1)}
}

func legacyConfig(available int64) map[string]any {
	return map[string]any{
		"configID": int64(7001), "name": "Legacy", "typeName": "Generic",
		"trees": []any{
			map[string]any{"name": "Professions", "points": int64(0)},
			map[string]any{"name": "Adventure", "points": int64(5)},
		},
		"currencies": []any{map[string]any{"available": available, "spent": int64(5), "max": int64(16), "totalEarned": int64(8)}},
		"nodes": []any{
			node("Well Rested", "Adventure", 5, 5, false),
			node("Thrill of Adventure", "Adventure", 0, 5, true),
			node("Working Overtime", "Professions", 0, 5, true),
			map[string]any{"name": "Talented", "tree": "Adventure", "rank": int64(0), "maxRank": int64(5), "canPurchase": false, "requires": []any{"Well Rested"}},
		},
	}
}

func snap(configs ...any) snapshot.Snapshot {
	return snapshot.Snapshot{"legacy": map[string]any{"configs": configs, "discovery": map[string]any{"globals": []any{"X"}}}}
}

func TestViewStatusesAndCanLearnNow(t *testing.T) {
	out := View(snap(legacyConfig(3)), Query{})
	canLearn := out["canLearnNow"].([]any)
	if len(canLearn) != 2 {
		t.Fatalf("canLearnNow = %v", canLearn)
	}
	trees := out["trees"].([]any)
	if first := trees[0].(map[string]any)["tree"]; first != "Professions" {
		t.Fatalf("tree order: first %v", first)
	}
	adventure := trees[1].(map[string]any)["perks"].([]any)
	statuses := map[any]any{}
	for _, raw := range adventure {
		p := raw.(map[string]any)
		statuses[p["name"]] = p["status"]
	}
	want := map[any]any{"Well Rested": string(StatusMaxed), "Thrill of Adventure": string(StatusAvailable), "Talented": string(StatusLocked)}
	for name, status := range want {
		if statuses[name] != status {
			t.Errorf("%v: status %v, want %v", name, statuses[name], status)
		}
	}
	points := out["points"].(map[string]any)
	if points["unspent"] != int64(3) || points["max"] != int64(16) || points["accountTotalEarned"] != int64(8) {
		t.Errorf("points = %v", points)
	}
	if _, ok := out["discovery"]; ok {
		t.Error("discovery should be hidden when a tree was found")
	}
}

func TestViewNoPointsEmptiesCanLearnNow(t *testing.T) {
	out := View(snap(legacyConfig(0)), Query{})
	if len(out["canLearnNow"].([]any)) != 0 || !strings.Contains(out["note"].(string), "No unspent Legacy Points") {
		t.Fatalf("out = %v", out)
	}
}

func TestViewTreeFilter(t *testing.T) {
	out := View(snap(legacyConfig(3)), Query{Tree: "prof"})
	trees := out["trees"].([]any)
	if len(trees) != 1 || trees[0].(map[string]any)["tree"] != "Professions" {
		t.Fatalf("trees = %v", trees)
	}
}

func TestPickConfigPrefersLegacy(t *testing.T) {
	other := map[string]any{"configID": int64(9), "name": "Something", "trees": []any{map[string]any{"name": "Tab"}}}
	out := View(snap(other, legacyConfig(1)), Query{})
	if out["configID"] != int64(7001) {
		t.Fatalf("picked %v", out["configID"])
	}
	if others := out["otherTraitConfigs"].([]any); len(others) != 1 {
		t.Fatalf("others = %v", others)
	}
}

func TestViewWithoutTreeShowsDiscovery(t *testing.T) {
	two := []any{map[string]any{"configID": int64(1)}, map[string]any{"configID": int64(2)}}
	out := View(snap(two...), Query{})
	if _, ok := out["discovery"]; !ok || !strings.Contains(out["note"].(string), "no Legacy perk tree") {
		t.Fatalf("out = %v", out)
	}
}

func TestViewWithoutSection(t *testing.T) {
	out := View(snapshot.Snapshot{}, Query{})
	if !strings.Contains(out["note"].(string), "0.15.0") {
		t.Fatalf("out = %v", out)
	}
}

func TestChallenges(t *testing.T) {
	c := summariseChallenges([]Challenge{
		{Name: "A", Completed: true},
		{Name: "B", Description: "do B", Reward: "1 point"},
	})
	if c.Total != 2 || len(c.CompletedNames) != 1 || len(c.Open) != 1 || c.NotListed != 0 {
		t.Fatalf("challenges = %+v", c)
	}
	many := make([]Challenge, maxOpenChallenges+3)
	if c := summariseChallenges(many); len(c.Open) != maxOpenChallenges || c.NotListed != 3 {
		t.Fatalf("truncation = %+v", c)
	}
	if empty := summariseChallenges(nil).render(); empty["note"] == nil {
		t.Fatalf("empty = %v", empty)
	}
}

func TestAnalyseRules(t *testing.T) {
	r := Analyse(snap(legacyConfig(0)), Query{})
	if r.Advice != AdviceNoPoints || len(r.CanLearn) != 0 || !r.Points.Known || r.Points.Unspent != 0 {
		t.Fatalf("no points: %+v", r)
	}
	r = Analyse(snap(legacyConfig(3)), Query{Tree: "adv"})
	if len(r.CanLearn) != 1 || r.CanLearn[0].Perk.Name != "Thrill of Adventure" || len(r.Trees) != 1 {
		t.Fatalf("filtered: %+v", r)
	}
	if got := (Perk{Rank: 5, MaxRank: 5}).Status(); got != StatusMaxed {
		t.Fatalf("status = %v", got)
	}
}

// realShape is a config like the first real dump: no name, trees without groups ("tree <ID>"), one shared currency.
func realShape() map[string]any {
	cur := func(tree int64) any {
		return map[string]any{"currencyID": int64(4225), "treeID": tree, "available": int64(0), "spent": int64(1), "max": int64(1)}
	}
	return map[string]any{
		"configID": int64(8159448), "name": "", "typeName": "Generic",
		"trees": []any{
			map[string]any{"name": "tree 1118", "treeID": int64(1118), "points": int64(0)},
			map[string]any{"name": "tree 1187", "treeID": int64(1187), "points": int64(0)},
			map[string]any{"name": "tree 1188", "treeID": int64(1188), "points": int64(1)},
		},
		"currencies": []any{cur(1187), cur(1188), cur(1189)},
		"nodes": []any{
			node("Working Overtime", "tree 1187", 0, 5, true),
			node("Thrill of Adventure", "tree 1188", 1, 5, true),
			node("Unknown", "tree 1188", 0, 1, false),
		},
	}
}

func TestRealShapeNamesTreesAndSharesCurrency(t *testing.T) {
	other := map[string]any{"configID": int64(9), "name": "", "trees": []any{map[string]any{"name": "tree 5", "treeID": int64(5)}}}
	out := View(snap(other, realShape()), Query{Tree: "adventure"})
	if out["configID"] != int64(8159448) {
		t.Fatalf("picked %v", out["configID"])
	}
	trees := out["trees"].([]any)
	if len(trees) != 1 || trees[0].(map[string]any)["tree"] != "Adventure" {
		t.Fatalf("trees = %v", trees)
	}
	perks := trees[0].(map[string]any)["perks"].([]any)
	var unknown map[string]any
	for _, raw := range perks {
		if p := raw.(map[string]any); p["name"] == "Unknown" {
			unknown = p
		}
	}
	if unknown == nil || unknown["unrevealed"] != true {
		t.Fatalf("unknown perk = %v", unknown)
	}
	points := out["points"].(map[string]any)
	if points["unspent"] != int64(0) || points["spent"] != int64(1) || points["max"] != int64(1) || points["otherCurrencies"] != nil {
		t.Fatalf("points = %v", points)
	}
	if len(out["canLearnNow"].([]any)) != 0 {
		t.Fatalf("no unspent points, canLearnNow = %v", out["canLearnNow"])
	}
}

func TestTreeNameKeepsRealNames(t *testing.T) {
	if got := treeName("Adventure", int64(42)); got != "Adventure" {
		t.Fatal(got)
	}
	if got := treeName("tree 1189", int64(1189)); got != "Resourcefulness" {
		t.Fatal(got)
	}
	if got := treeName("tree 77", int64(77)); got != "tree 77" {
		t.Fatal(got)
	}
}
