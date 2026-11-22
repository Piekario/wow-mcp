// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questroute

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func q(title, zone string, level int64, complete bool) any {
	return map[string]any{"title": title, "zone": zone, "level": level, "complete": complete}
}

func snap(zone string, level int64, flights []any, qs ...any) snapshot.Snapshot {
	s := snapshot.Snapshot{
		"character": map[string]any{"zone": zone, "level": level},
		"quests":    qs,
	}
	if flights != nil {
		s["flightPaths"] = map[string]any{"nodes": flights}
	}
	return s
}

func zonesOf(t *testing.T, out map[string]any) []string {
	t.Helper()
	var names []string
	for _, r := range out["route"].([]any) {
		names = append(names, r.(map[string]any)["zone"].(string))
	}
	return names
}

func TestRouteOrder(t *testing.T) {
	tests := []struct {
		name string
		s    snapshot.Snapshot
		want []string
	}{
		{"current zone first despite a smaller count", snap("B", 20, nil,
			q("a", "A", 20, true), q("a2", "A", 20, false), q("b", "B", 20, false)), []string{"B", "A"}},
		{"ready to turn in matters more than the count", snap("X", 20, nil,
			q("a", "A", 20, false), q("a2", "A", 20, false), q("b", "B", 20, true)), []string{"B", "A"}},
		{"tie broken alphabetically", snap("X", 20, nil, q("z", "Z", 20, false), q("a", "A", 20, false)), []string{"A", "Z"}},
		{"a zone with only red quests goes last", snap("X", 20, nil,
			q("r", "Red", 30, false), q("r2", "Red", 31, false), q("g", "Ok", 20, false)), []string{"Ok", "Red"}},
		{"a red zone with a ready quest does not drop to the end", snap("X", 20, nil,
			q("r", "Red", 30, false), q("r2", "Red", 31, true), q("g", "Ok", 20, false)), []string{"Red", "Ok"}},
		{"the current zone with only red quests is still first", snap("Red", 20, nil,
			q("r", "Red", 30, false), q("g", "Ok", 20, false)), []string{"Red", "Ok"}},
		{"case-insensitive", snap("a", 20, nil, q("b", "B", 20, false), q("a", "A", 20, false)), []string{"A", "B"}},
		{"empty log", snap("A", 20, nil), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := zonesOf(t, View(tc.s, fixtures.RealTime()))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestRedQuestsMarkedAndLast(t *testing.T) {
	out := View(snap("X", 20, nil, q("hard", "A", 26, false), q("easy", "A", 20, false), q("done", "A", 26, true)), fixtures.RealTime())
	r := out["route"].([]any)[0].(map[string]any)
	titles := []string{}
	for _, e := range r["quests"].([]any) {
		m := e.(map[string]any)
		titles = append(titles, m["title"].(string))
		if (m["title"] == "hard") != (m["doLast"] == true) {
			t.Errorf("doLast wrong for %v", m)
		}
	}
	if titles[0] != "done" || titles[1] != "easy" || titles[2] != "hard" {
		t.Errorf("quest order: %v", titles)
	}
	if r["tooHard"] != 1 || r["readyToTurnIn"] != 1 || r["inProgress"] != 1 {
		t.Errorf("counters: %v", r)
	}
}

func TestFlightPointsAndHints(t *testing.T) {
	flights := []any{
		map[string]any{"name": "Auberdine, Darkshore", "state": "current"},
		map[string]any{"name": "Ironforge", "state": "reachable"},
		map[string]any{"name": "Elsewhere", "state": "x", "zone": "B"},
	}
	out := View(snap("X", 20, flights, q("a", "Darkshore", 20, false), q("b", "Ironforge", 20, false), q("c", "B", 20, false), q("d", "None", 20, false)), fixtures.RealTime())
	got := map[string]int{}
	for _, r := range out["route"].([]any) {
		m := r.(map[string]any)
		fp, _ := m["flightPoints"].([]any)
		got[m["zone"].(string)] = len(fp)
	}
	if got["Darkshore"] != 1 || got["Ironforge"] != 1 || got["B"] != 1 || got["None"] != 0 {
		t.Errorf("flight matching: %v", got)
	}
	if _, ok := out["flightHint"]; ok {
		t.Error("hint present despite the flightPaths section")
	}
	if _, ok := View(snap("X", 20, nil, q("a", "A", 20, false)), fixtures.RealTime())["flightHint"]; !ok {
		t.Error("missing hint without flightPaths")
	}
}

func TestAssumptionAndMissingQuests(t *testing.T) {
	out := View(snapshot.Snapshot{"character": map[string]any{"zone": "A", "level": int64(10)}}, fixtures.RealTime())
	if out["assumption"] == "" || out["note"] == nil {
		t.Errorf("%v", out)
	}
	out = View(snap("Nowhere", 20, nil, q("a", "A", 20, false)), fixtures.RealTime())
	if out["currentZoneNote"] == nil {
		t.Error("missing note about the current zone without quests")
	}
	if len(out["assumption"].(string)) == 0 {
		t.Error("assumption")
	}
}

func TestRealSnapshot(t *testing.T) {
	out := View(fixtures.Real(t), fixtures.RealTime())
	names := zonesOf(t, out)
	if len(names) < 2 {
		t.Fatalf("zones: %v", names)
	}
	total := 0
	for _, r := range out["route"].([]any) {
		total += r.(map[string]any)["total"].(int)
	}
	if out["count"] != total || total == 0 {
		t.Errorf("count %v vs %d", out["count"], total)
	}
	if out["currentZone"] != "Westfall" {
		t.Errorf("currentZone = %v", out["currentZone"])
	}
	// The flight window from Westfall knows only Eastern Kingdoms: Darkshore has no point, and the shortened names
	// (Lakeshire, Redridge; Stormwind, Elwynn) match the full zone names from the quests.
	points := map[string]bool{}
	for _, r := range out["route"].([]any) {
		m := r.(map[string]any)
		points[m["zone"].(string)] = m["flightPoints"] != nil
	}
	for zone, want := range map[string]bool{"Westfall": true, "Redridge Mountains": true, "Stormwind City": true, "Darkshore": false} {
		if points[zone] != want {
			t.Errorf("flight point in %s = %v, want %v", zone, points[zone], want)
		}
	}
}

func TestForeverFixtureNoPanic(t *testing.T) {
	out := View(fixtures.Forever(t), fixtures.At(0))
	if out["assumption"] == nil {
		t.Error("assumption")
	}
}

func TestRouteByMapID(t *testing.T) {
	// A German client: zone names do not match the English table, the map IDs do.
	qs := []any{
		map[string]any{"title": "a", "zone": "Dunkelküste", "zoneMapID": int64(1439), "level": int64(20)},
		map[string]any{"title": "b", "zone": "Teldrassil", "zoneMapID": int64(1438), "level": int64(20)},
		map[string]any{"title": "c", "zone": "Teldrassil", "zoneMapID": int64(1438), "level": int64(20)},
	}
	nodes := []any{map[string]any{"name": "Auberdine, Dunkelküste", "state": "reachable", "zone": "Dunkelküste", "zoneMapID": int64(1439)}}
	s := snap("Other name", 20, nodes, qs...)
	s["character"].(map[string]any)["mapID"] = int64(1439)
	out := View(s, fixtures.RealTime())
	route := out["route"].([]any)
	first := route[0].(map[string]any)
	if first["zone"] != "Dunkelküste" || first["current"] != true {
		t.Errorf("the character's map ID makes the zone current: %v", first)
	}
	points := first["flightPoints"].([]any)
	if len(points) != 1 || points[0].(map[string]any)["matchedBy"] != "zone map ID" {
		t.Errorf("flight points = %v", points)
	}
	if route[1].(map[string]any)["flightPoints"] != nil {
		t.Errorf("Teldrassil has no point: %v", route[1])
	}
}
