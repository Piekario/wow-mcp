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
	want := map[any]any{"Well Rested": StatusMaxed, "Thrill of Adventure": StatusAvailable, "Talented": StatusLocked}
	for name, status := range want {
		if statuses[name] != status {
			t.Errorf("%v: status %v, want %v", name, statuses[name], status)
		}
	}
	points := out["points"].(map[string]any)
	if points["unspent"] != int64(3) || points["characterCap"] != int64(16) || points["accountTotalEarned"] != int64(8) {
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
	c := challenges(map[string]any{"list": []any{
		map[string]any{"name": "A", "completed": true},
		map[string]any{"name": "B", "completed": false, "description": "do B", "reward": "1 point"},
	}})
	if c["total"] != 2 || c["completed"] != 1 || len(c["open"].([]any)) != 1 {
		t.Fatalf("challenges = %v", c)
	}
	if empty := challenges(map[string]any{"list": []any{}}); empty["note"] == nil {
		t.Fatalf("empty = %v", empty)
	}
}
