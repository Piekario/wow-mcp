// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package professions

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestRecipes(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := fixtures.At(10 * time.Minute)
	r := Recipes(s, RecipeQuery{Profession: "leather"}, fresh)
	text := fixtures.JSON(t, r)
	fixtures.WantAll(t, "recipes", text, `"rank":87`, `"trivial":1`, `"missingForOne":["Coarse Thread 2"]`, `"cooldown":"11 h 50 min"`)
	if strings.Contains(fixtures.JSON(t, Recipes(withBeastTraining(s), RecipeQuery{Profession: "leather"}, fresh)), "Beast Training") {
		t.Error("the profession filter let Beast Training through")
	}
	if up := Recipes(s, RecipeQuery{SkillUp: true, Profession: "leather"}, fresh); strings.Contains(fixtures.JSON(t, up), "Light Armor Kit") {
		t.Error("skillUp should skip a gray recipe")
	}
	if c := Recipes(s, RecipeQuery{Craftable: true}, fresh); !strings.Contains(fixtures.JSON(t, c), "Light Armor Kit") || strings.Contains(fixtures.JSON(t, c), "Embossed") {
		t.Errorf("craftable: %s", fixtures.JSON(t, c))
	}
	// Only older clients have the Craft window (Beast Training); Forever does not, so the section is set directly.
	s["recipes"].(map[string]any)["Beast Training"] = map[string]any{"name": "Beast Training", "recipes": []any{
		map[string]any{"name": "Bite", "rank": "Rank 3", "difficulty": "available", "trainingPoints": int64(7), "levelRequired": int64(24)},
	}}
	bt := Recipes(s, RecipeQuery{Profession: "beast"}, fresh)
	fixtures.WantAll(t, "beast training", fixtures.JSON(t, bt), `"trainingPoints":7`, `"levelRequired":24`)
	if n := Recipes(snapshot.Snapshot{}, RecipeQuery{}, fresh); n["note"] == nil {
		t.Error("no note without recipes")
	}
}

func withBeastTraining(s snapshot.Snapshot) snapshot.Snapshot {
	s["recipes"].(map[string]any)["Beast Training"] = map[string]any{"name": "Beast Training", "recipes": []any{}}
	return s
}

// A material without a name (an item not loaded in the client) gets "item <id>", not an empty string.
func TestReagentWithoutName(t *testing.T) {
	s := fixtures.Forever(t)
	lw := s["recipes"].(map[string]any)["Leatherworking"].(map[string]any)
	for _, raw := range lw["recipes"].([]any) {
		for _, rr := range snapshot.AsList(snapshot.AsMap(raw)["reagents"]) {
			delete(snapshot.AsMap(rr), "name")
		}
	}
	text := fixtures.JSON(t, Recipes(s, RecipeQuery{Profession: "leather"}, fixtures.At(0)))
	fixtures.WantAll(t, "without names", text, `"missingForOne":["item 2320 2"]`)
}

func TestProfessionSummarySkipsWeaponSkills(t *testing.T) {
	for _, p := range Summary(fixtures.Forever(t)) {
		if strings.HasPrefix(p, "Daggers") {
			t.Errorf("weapon skill in the profession list: %v", p)
		}
	}
}

func TestProfessionsByIDOnAGermanClient(t *testing.T) {
	s := snapshot.Snapshot{"professions": []any{
		map[string]any{"name": "Kürschnerei", "rank": int64(96), "maxRank": int64(150), "group": "Berufe", "skillID": int64(393)},
		map[string]any{"name": "Erste Hilfe", "rank": int64(10), "maxRank": int64(75), "group": "Sekundäre Fertigkeiten", "skillID": int64(129)},
		map[string]any{"name": "Dolche", "rank": int64(98), "maxRank": int64(100), "group": "weapon", "skillID": int64(173)},
		map[string]any{"name": "Rüstung", "group": "Rüstungsfertigkeiten"},
	}}
	if got := strings.Join(Summary(s), "; "); got != "Kürschnerei 96/150; Erste Hilfe 10/75" {
		t.Errorf("summary: %q", got)
	}
	if !HasSkill(s, SkillSkinning) || HasSkill(s, SkillMining) {
		t.Error("HasSkill by skillID")
	}
	old := snapshot.Snapshot{"professions": []any{map[string]any{"name": "Mining", "group": "Professions"}}}
	if !HasSkill(old, SkillMining) {
		t.Error("an old English dump without skillID: by name")
	}
}
