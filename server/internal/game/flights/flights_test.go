// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package flights

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func quest(title, zone string) any  { return map[string]any{"title": title, "zone": zone} }
func nodeOf(name, state string) any { return map[string]any{"name": name, "state": state} }

func snap(nodes []any, quests ...any) snapshot.Snapshot {
	return snapshot.Snapshot{
		"flightPaths": map[string]any{"capturedAt": int64(fixtures.ForeverDumpedAt), "nodes": nodes},
		"quests":      quests,
	}
}

func zones(out map[string]any, key string) []string {
	var zs []string
	for _, raw := range out[key].([]any) {
		zs = append(zs, raw.(map[string]any)["zone"].(string))
	}
	return zs
}

func TestView(t *testing.T) {
	nodes := []any{
		nodeOf("Auberdine, Darkshore", "current"),
		nodeOf("Moonglade", "unreachable"),
		map[string]any{"name": "Dun Morogh Post", "zone": "Dun Morogh"},
	}
	tests := []struct {
		name          string
		quests        []any
		with, without []string
	}{
		{"name suffix", []any{quest("A", "Darkshore"), quest("B", "Darkshore")}, []string{"Darkshore"}, []string{}},
		{"no point", []any{quest("A", "Westfall")}, []string{}, []string{"Westfall"}},
		{"whole name contains zone", []any{quest("A", "Moonglade")}, []string{"Moonglade"}, []string{}},
		{"zone field", []any{quest("A", "Dun Morogh")}, []string{"Dun Morogh"}, []string{}},
		{"letter case", []any{quest("A", "darkshore")}, []string{"darkshore"}, []string{}},
		{"quest without a zone", []any{map[string]any{"title": "X"}}, []string{}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := View(snap(nodes, tc.quests...), fixtures.At(0))
			if got := zones(out, "questZonesWithFlightPoint"); len(got) != len(tc.with) || (len(got) > 0 && got[0] != tc.with[0]) {
				t.Errorf("with = %v, want %v", got, tc.with)
			}
			if got := zones(out, "questZonesWithoutFlightPoint"); len(got) != len(tc.without) || (len(got) > 0 && got[0] != tc.without[0]) {
				t.Errorf("without = %v, want %v", got, tc.without)
			}
			if out["assumption"] == nil {
				t.Error("no assumption")
			}
		})
	}
}

func TestOrderByQuestCount(t *testing.T) {
	out := View(snap(nil, quest("a", "B"), quest("b", "A"), quest("c", "A"), quest("d", "C")), fixtures.At(0))
	got := zones(out, "questZonesWithoutFlightPoint")
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Errorf("zone order: %v", got)
	}
}

func TestMissingSection(t *testing.T) {
	out := View(snapshot.Snapshot{"quests": []any{quest("A", "X")}}, fixtures.At(0))
	if out["hint"] == nil {
		t.Fatalf("expected a hint: %v", out)
	}
	fixtures.WantAll(t, "hint", out["hint"].(string), "flight master", "/mcp reload")
	if out2 := View(snapshot.Snapshot{"flightPaths": "x", "quests": 5}, snapshot.Freshness{}); out2["hint"] == nil {
		t.Error("a bad section = same as missing")
	}
}

func TestReal(t *testing.T) {
	out := View(fixtures.Real(t), fixtures.RealTime())
	if n, _ := out["knownFlightPoints"].(int); n < 10 {
		t.Errorf("knownFlightPoints = %v", out["knownFlightPoints"])
	}
	if out["capturedAt"] == nil {
		t.Error("no capturedAt")
	}
	// 18 entries in the dump, one of them retired (zzOLDRiverglades, Farholde Keep).
	if out["knownFlightPoints"] != 17 || out["withoutNote"] == nil {
		t.Errorf("points without zzOLD: %v, withoutNote: %v", out["knownFlightPoints"], out["withoutNote"])
	}
}

func TestMatchZoneExported(t *testing.T) {
	nodes := []Node{
		{Name: "Auberdine, Darkshore", State: "current", Zone: "Darkshore"},
		{Name: "Astranaar, Ashenvale"},
		{Name: "Menethil Harbor"},
		{Name: "Lakeshire, Redridge"},
		{Name: "Stormwind, Elwynn"},
	}
	cases := map[string][]string{
		"darkshore": {byZoneField}, "Ashenvale": {byNameSuffix}, "Menethil": {byNameContain}, "Elwynn": {byNameSuffix}, "Tanaris": nil,
		"Redridge Mountains": {byShortName}, "Stormwind City": {byShortName}, "Elwynn Forest": {byShortName},
		"Red": nil, "Storm": nil,
	}
	for zone, wantBy := range cases {
		got := MatchZone(zone, nodes)
		if len(got) != len(wantBy) || (len(got) == 1 && got[0].By != wantBy[0]) {
			t.Errorf("MatchZone(%q) = %+v, want %v", zone, got, wantBy)
		}
	}
}

func TestMatchZoneIDFirst(t *testing.T) {
	nodes := []Node{
		{Name: "Auberdine, Darkshore", Zone: "Darkshore", ZoneMapID: 1439, NodeID: 26},
		{Name: "Rut'theran Village, Teldrassil", Zone: "Teldrassil", ZoneMapID: 1438},
		{Name: "Astranaar, Ashenvale"}, // an older dump: no IDs
	}
	cases := []struct {
		zone   string
		id     int64
		wantBy []string
	}{
		{"Dunkelküste", 1439, []string{byZoneMapID}}, // German zone name: the ID alone matches
		{"Darkshore", 9999, nil},                     // a point with an ID of another zone is never matched by name
		{"Darkshore", 0, []string{byZoneField}},      // a quest without an ID falls back to the name
		{"Ashenvale", 1440, []string{byNameSuffix}},  // a point without an ID falls back to the name
		{"Elwynn Forest", 1429, nil},
	}
	for _, tc := range cases {
		got := MatchZoneID(tc.zone, tc.id, nodes)
		if len(got) != len(tc.wantBy) || (len(got) == 1 && got[0].By != tc.wantBy[0]) {
			t.Errorf("MatchZoneID(%q, %d) = %+v, want %v", tc.zone, tc.id, got, tc.wantBy)
		}
	}
}

func TestViewMatchesByMapID(t *testing.T) {
	nodes := []any{map[string]any{"name": "Auberdine, Dunkelküste", "state": "current", "zone": "Dunkelküste", "zoneMapID": int64(1439), "nodeID": int64(26)}}
	quests := []any{map[string]any{"title": "Pilze", "zone": "Dunkelküste", "zoneMapID": int64(1439)}, quest("Other", "Westfall")}
	s := snap(nodes, quests...)
	s["character"] = map[string]any{"locale": "deDE"}
	out := View(s, fixtures.At(0))
	if got := zones(out, "questZonesWithFlightPoint"); len(got) != 1 || got[0] != "Dunkelküste" {
		t.Errorf("with = %v", got)
	}
	point := out["questZonesWithFlightPoint"].([]any)[0].(map[string]any)["flightPoints"].([]any)[0].(map[string]any)
	if point["matchedBy"] != byZoneMapID {
		t.Errorf("matchedBy = %v", point["matchedBy"])
	}
	if out["localeNote"] != nil {
		t.Errorf("every point has a zone ID, so no name matching and no locale note: %v", out["localeNote"])
	}
	// A point with neither a zone ID nor a zone field is matched by name: a German client then gets the note.
	s = snap([]any{map[string]any{"name": "Astranaar, Ashenvale"}}, quest("A", "Ashenvale"))
	s["character"] = map[string]any{"locale": "deDE"}
	if View(s, fixtures.At(0))["localeNote"] == nil {
		t.Error("name matching on a German client needs the localeNote")
	}
}

func TestParseNodesIDs(t *testing.T) {
	nodes := ParseNodes([]any{map[string]any{"name": "A", "nodeID": int64(7), "zoneMapID": int64(1439)}})
	if len(nodes) != 1 || nodes[0].NodeID != 7 || nodes[0].ZoneMapID != 1439 {
		t.Errorf("nodes = %+v", nodes)
	}
}

// Real dump of 2026-11-16 (addon 0.11.0, flight map captured by addon 0.7.0): every point has zone = "Eastern Kingdoms",
// the continent. Such a zone counts as none, and the names decide.
func TestViewRealContinentZones(t *testing.T) {
	s := fixtures.Real0110(t)
	nodes := ParseNodes(snapshot.AsList(s.Map("flightPaths")["nodes"]))
	if len(nodes) != 17 || nodes[0].Zone != "" || nodes[0].Continent != "Eastern Kingdoms" {
		t.Fatalf("nodes: %d, first %+v", len(nodes), nodes[0])
	}
	out := View(s, fixtures.RealTime())
	with := zones(out, "questZonesWithFlightPoint")
	want := map[string]bool{"Westfall": true, "Redridge Mountains": true, "Duskwood": true, "Loch Modan": true,
		"Wetlands": true, "Dun Morogh": true, "Stormwind City": true}
	if len(with) != len(want) {
		t.Fatalf("with = %v", with)
	}
	for _, z := range with {
		if !want[z] {
			t.Errorf("unexpected zone with a flight point: %s", z)
		}
	}
	without := zones(out, "questZonesWithoutFlightPoint")
	if len(without) != 2 || without[0] != "Darkshore" || without[1] != "The Deadmines" || out["withoutNote"] == nil {
		t.Errorf("without = %v", without)
	}
}

func TestGuessableKeepsMatchingZonesAndRetriesTheRest(t *testing.T) {
	nodes := []Node{
		{Name: "Auberdine, Darkshore", Zone: "Darkshore", ZoneMapID: 1439},
		{Name: "Sentinel Hill, Westfall", Zone: "Elwynn Forest", ZoneMapID: 1429}, // a wrong zone: matches no quest zone
		{Name: "Lakeshire, Redridge", Continent: "Eastern Kingdoms"},
	}
	got := QuestZones{"Darkshore": 1439, "Westfall": 1436}.Guessable(nodes)
	if got[0].ZoneUnmatched || got[0].Zone != "Darkshore" {
		t.Errorf("a matching zone stays: %+v", got[0])
	}
	if !got[1].ZoneUnmatched || got[1].Zone != "" || got[1].ZoneMapID != 0 || nodes[1].Zone != "Elwynn Forest" {
		t.Errorf("an unmatched zone is cleared on a copy: %+v (input %+v)", got[1], nodes[1])
	}
	if got[2].ZoneUnmatched {
		t.Errorf("a node without a zone is not marked: %+v", got[2])
	}
	m := MatchZoneID("Westfall", 1436, got)
	if len(m) != 1 || m[0].By != byNameSuffix || m[0].View()["zoneFieldUnmatched"] != true {
		t.Errorf("Westfall: %+v", m)
	}
	if n := ParseNodes([]any{map[string]any{"name": "X, Y", "zone": "Kalimdor"}, map[string]any{"name": "Z", "continent": "Northrend"}}); n[0].Zone != "" || n[0].Continent != "Kalimdor" || n[1].Continent != "Northrend" {
		t.Errorf("continents: %+v", n)
	}
}
