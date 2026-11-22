// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questguide

import (
	"strings"
	"testing"

	"wowmcp/internal/snapshot"
)

func npc(id int64, name, zone string, x, y float64) map[string]any {
	return map[string]any{"kind": "npc", "id": id, "name": name, "spawnCount": int64(1),
		"locations": []any{map[string]any{"zone": zone, "areaID": int64(17), "x": x, "y": y}}}
}

// dump is a character standing in The Barrens with one quest ready to turn in, one in progress and one finished.
func dump() snapshot.Snapshot {
	shard := map[string]any{"id": int64(6981), "name": "The Glowing Shard", "level": int64(26), "inLog": true,
		"objectivesText": "Travel to Ratchet to find someone that can tell you more about the glowing shard.",
		"startedBy":      []any{map[string]any{"kind": "item", "id": int64(10441), "name": "Glowing Shard"}},
		"finishedBy":     []any{npc(8418, "Falla Sagewind", "The Barrens", 48.2, 32.8)},
		"nextQuest":      map[string]any{"id": int64(3369), "name": "In Nightmares"}}
	hides := map[string]any{"id": int64(1486), "name": "Deviate Hides", "inLog": true, "inGuide": true,
		"finishedBy": []any{npc(5768, "Nalpak", "The Barrens", 47, 36.3)},
		"objectives": []any{map[string]any{"kind": "npc", "id": int64(3633), "name": "Deviate Slayer", "spawnCount": int64(9), "zoneCount": int64(1),
			"locations": []any{map[string]any{"zone": "Wailing Caverns", "areaID": int64(718), "inInstance": true}}}}}
	later := map[string]any{"id": int64(3369), "name": "In Nightmares", "inGuide": true,
		"startedBy": []any{npc(8418, "Falla Sagewind", "The Barrens", 48.2, 32.8)}}
	return snapshot.Snapshot{
		"character": map[string]any{"zone": "The Barrens", "x": 46.0, "y": 36.0},
		"quests": []any{
			map[string]any{"id": int64(6981), "title": "The Glowing Shard", "complete": true},
			map[string]any{"id": int64(1486), "title": "Deviate Hides", "complete": false},
		},
		"completedQuests": []any{int64(959)},
		"questDb": map[string]any{
			"source": map[string]any{"name": "QuestieDB"},
			"quests": []any{shard, hides, later},
		},
		"questLookups": []any{map[string]any{"query": "sputtervalve", "at": int64(1),
			"quests": []any{},
			"npcs":   []any{npc(3442, "Sputtervalve", "The Barrens", 63, 37.2)}}},
		"guide": map[string]any{"addon": "Guidelime", "name": "TUGs 20-22 Wailing Caverns", "nextGuide": "22-24 Hillsbrad Foothills",
			"totalSteps": int64(30), "doneSteps": int64(12),
			"next": []any{
				map[string]any{"line": int64(13), "text": "[O]Do: [QC1486- ]", "optional": true, "passed": true,
					"quests": []any{map[string]any{"id": int64(1486), "action": "complete"}}},
				map[string]any{"line": int64(21), "text": "Run to [G63, 37.2The Barrens][A Undead]"},
				map[string]any{"line": int64(30), "text": "TurnIn: [QT6981]",
					"quests": []any{map[string]any{"id": int64(6981), "action": "turnin"}}},
				map[string]any{"line": int64(31), "text": "Accept: [QA3369] then [QA777]",
					"quests": []any{map[string]any{"id": int64(3369), "action": "accept"}, map[string]any{"id": int64(777), "action": "accept"}}},
				map[string]any{"line": int64(35), "text": "Use your [H] Hearthstone, then fly to [F Orgrimmar] and get the [P] flightpath"},
			}},
	}
}

func questByID(t *testing.T, out map[string]any, id int64) map[string]any {
	t.Helper()
	for _, raw := range snapshot.AsList(out["quests"]) {
		q := snapshot.AsMap(raw)
		if got, _ := snapshot.AsInt(q["id"]); got == id {
			return q
		}
	}
	t.Fatalf("quest %d not in the answer: %v", id, out["quests"])
	return nil
}

func TestLookupListsTheLogWithTheNextStep(t *testing.T) {
	out := Lookup(dump(), Query{})
	if out["count"] != 2 {
		t.Fatalf("the log has two quests, got %v", out["count"])
	}
	shard := questByID(t, out, 6981)
	if got := shard["nextStep"]; got != "Turn in at Falla Sagewind (The Barrens 48.2, 32.8)" {
		t.Errorf("nextStep = %v", got)
	}
	if _, has := shard["objectives"]; has {
		t.Error("a quest ready to turn in needs no objective targets")
	}
	if _, has := shard["pickUp"]; has {
		t.Error("a quest in the log needs no pick-up place")
	}
	where := snapshot.AsMap(snapshot.AsList(snapshot.AsMap(snapshot.AsList(shard["turnIn"])[0])["where"])[0])
	if got := where["fromPlayer"]; got != "3.9 map units north-east of the character" {
		t.Errorf("fromPlayer = %v", got)
	}
	hides := questByID(t, out, 1486)
	if got := hides["nextStep"]; got != "Finish the objectives, then turn in at Nalpak (The Barrens 47, 36.3)" {
		t.Errorf("nextStep = %v", got)
	}
	target := snapshot.AsMap(snapshot.AsList(hides["objectives"])[0])
	if got := target["note"]; got != "9 spawn points in total; 1 shown" {
		t.Errorf("objective note = %v", got)
	}
	if loc := snapshot.AsMap(snapshot.AsList(target["where"])[0]); loc["inInstance"] != true || loc["x"] != nil {
		t.Errorf("an instance location has no coordinates: %v", loc)
	}
}

func TestLookupByNameIDAndNpc(t *testing.T) {
	out := Lookup(dump(), Query{Quest: "nightmares"})
	later := questByID(t, out, 3369)
	if later["status"] != statusNotInLog || later["nextStep"] != "Pick up from Falla Sagewind (The Barrens 48.2, 32.8)" {
		t.Errorf("a guide quest not yet accepted: %v", later)
	}
	if out := Lookup(dump(), Query{Quest: " 6981 "}); out["count"] != 1 {
		t.Errorf("lookup by ID: %v", out["count"])
	}
	out = Lookup(dump(), Query{Quest: "Sputtervalve"})
	npcs := snapshot.AsList(out["npcs"])
	if len(npcs) != 1 || snapshot.AsMap(npcs[0])["name"] != "Sputtervalve" {
		t.Fatalf("an NPC from /mcp quest: %v", out)
	}
	if _, has := out["hint"]; has {
		t.Error("an NPC match is an answer, not a miss")
	}
}

func TestLookupNeverGuesses(t *testing.T) {
	out := Lookup(dump(), Query{Quest: "Zzzz"})
	if hint, _ := out["hint"].(string); !strings.Contains(hint, "/mcp quest") || out["count"] != 0 {
		t.Errorf("a miss must say how to get the data: %v", out)
	}
	out = Lookup(snapshot.Snapshot{"quests": []any{}}, Query{})
	if hint, _ := out["hint"].(string); out["available"] != false || !strings.Contains(hint, "Do NOT answer") {
		t.Errorf("no database: %v", out)
	}
}

func TestCompass(t *testing.T) {
	cases := []struct {
		dx, dy float64
		want   string
	}{{1, 0, "east"}, {0, 1, "south"}, {-1, 0, "west"}, {0, -1, "north"}, {1, -1, "north-east"}, {-3, 3, "south-west"}, {-2, -2, "north-west"}, {5, 5, "south-east"}}
	for _, c := range cases {
		if got := compass(c.dx, c.dy); got != c.want {
			t.Errorf("compass(%g, %g) = %s, want %s", c.dx, c.dy, got, c.want)
		}
	}
}

func TestGuideStepsAreReadableAndLocated(t *testing.T) {
	out := Guide(dump())
	if out["guide"] != "TUGs 20-22 Wailing Caverns" || out["shownSteps"] != 5 {
		t.Fatalf("guide header: %v", out)
	}
	if got := snapshot.AsMap(out["progress"])["remainingSteps"]; got != int64(18) {
		t.Errorf("remainingSteps = %v", got)
	}
	steps := snapshot.AsList(out["steps"])
	texts := []string{
		`Do: "Deviate Hides"`,
		"Run to 63, 37.2 The Barrens",
		`TurnIn: "The Glowing Shard"`,
		`Accept: "In Nightmares" then quest 777`,
		"Use your Hearthstone, then fly to Orgrimmar and get the flightpath",
	}
	for i, want := range texts {
		if got := snapshot.AsMap(steps[i])["text"]; got != want {
			t.Errorf("step %d text = %q, want %q", i, got, want)
		}
	}
	first := snapshot.AsMap(steps[0])
	if first["optional"] != true || first["passedEarlier"] != true {
		t.Errorf("flags of the first step: %v", first)
	}
	turnIn := snapshot.AsMap(snapshot.AsList(snapshot.AsMap(steps[2])["quests"])[0])
	if turnIn["status"] != statusReady || snapshot.AsMap(snapshot.AsList(turnIn["where"])[0])["name"] != "Falla Sagewind" {
		t.Errorf("turn-in step: %v", turnIn)
	}
	unknown := snapshot.AsMap(snapshot.AsList(snapshot.AsMap(steps[3])["quests"])[1])
	if note, _ := unknown["note"].(string); !strings.Contains(note, "/mcp quest 777") {
		t.Errorf("a quest missing from the database: %v", unknown)
	}
}

func TestGuideWithoutData(t *testing.T) {
	if out := Guide(snapshot.Snapshot{}); out["available"] != false || out["hint"] != noGuideHint {
		t.Errorf("no guide section: %v", out)
	}
	s := snapshot.Snapshot{"guide": map[string]any{"addon": "Guidelime", "name": "Custom 1-5", "textUnavailable": true}}
	if out := Guide(s); out["available"] != false || out["hint"] != noGuideTextHint || out["guide"] != "Custom 1-5" {
		t.Errorf("guide text not seen: %v", out)
	}
	s = snapshot.Snapshot{"guide": map[string]any{"name": "Done 1-5", "totalSteps": int64(4), "doneSteps": int64(4), "next": []any{}}}
	if hint, _ := Guide(s)["hint"].(string); !strings.Contains(hint, "next guide") {
		t.Errorf("a finished guide: %q", hint)
	}
}

func TestGuideCountsPassedSteps(t *testing.T) {
	s := snapshot.Snapshot{"guide": map[string]any{
		"name": "Mock 1-2", "totalSteps": int64(23), "doneSteps": int64(1), "passedSteps": int64(20),
		"next": []any{map[string]any{"line": int64(2), "text": "Accept: [QA1]", "passed": true}},
	}}
	if got := Guide(s)["passedEarlierSteps"]; got != int64(20) {
		t.Fatalf("passedEarlierSteps = %v, want 20", got)
	}
}
