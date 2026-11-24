// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestViewOnFixture(t *testing.T) {
	s := fixtures.Forever(t)
	if !Exists(s) {
		t.Fatal("the fixture has a pet")
	}
	text := fixtures.JSON(t, View(s))
	fixtures.WantAll(t, "pet", text, `"name":"Cat"`, `"family":"Cat"`, `"happiness":{"name":"Happy","value":3}`,
		`"damagePercent":125`, `"xpPercent":27.9`, `"abilities":["Growl Rank 3","Claw Rank 2"]`, `"autoCast":{"Claw":false,"Growl":true}`)
	if View(s)["trainingPoints"] != nil {
		t.Error("the fixture has no training points")
	}
}

func TestTrainingPointsAndFood(t *testing.T) {
	s := fixtures.Forever(t)
	p := snapshot.AsMap(s["pet"])
	p["trainingPointsTotal"], p["trainingPointsSpent"], p["trainingPointsFree"] = int64(20), int64(13), int64(7)
	p["foodTypes"] = []any{"Meat", "Fish"}
	fixtures.WantAll(t, "points", fixtures.JSON(t, View(s)), `"trainingPoints":{"free":7,"spent":13,"total":20}`, `"foodTypes":["Meat","Fish"]`)

	// Food & Drink by item subtype: the cooked meat and the juice, not the raw meat (Trade Goods).
	if food := FoodInBags(s); len(food) != 2 || snapshot.AsMap(food[0])["name"] != "Haunch of Meat" {
		t.Errorf("fixture food: %v", food)
	}
	bag := snapshot.AsMap(snapshot.AsList(s["bags"])[0])
	bag["items"] = append(snapshot.AsList(bag["items"]), map[string]any{"name": "Tough Jerky", "id": int64(117), "count": int64(8), "subType": "Food & Drink"})
	food := FoodInBags(s)
	if len(food) != 3 || snapshot.AsMap(food[0])["name"] != "Tough Jerky" || snapshot.AsMap(food[0])["count"] != int64(8) {
		t.Errorf("food: %v", food)
	}
}

func TestMissingOrDismissedPet(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s, "pet")
	v := View(s)
	if v["exists"] != false || v["note"] != NoPetNote || Exists(s) {
		t.Errorf("without the section: %v", v)
	}
	s["pet"] = map[string]any{"exists": false}
	v = View(s)
	if v["exists"] != false || v["note"] == NoPetNote || v["note"] == nil {
		t.Errorf("pet not summoned: %v", v)
	}
	s["pet"] = map[string]any{"exists": true, "happinessInfo": map[string]any{"state": "happy"}}
	if View(s)["happinessInfo"] == nil {
		t.Error("the new happiness format should be passed through")
	}
}

// The real dump has stances and commands with huge IDs in the abilities list too; they are not abilities.
func TestAbilitiesSkipStancesAndCommands(t *testing.T) {
	names := abilityLabels(Inspect(fixtures.Real(t)).Learned())
	want := []string{"Arcane Resistance Rank 1", "Cower Rank 1", "Faster Attack V", "Fire Resistance Rank 1", "Frost Resistance Rank 1",
		"Great Stamina Rank 3", "Growl Rank 3", "Natural Armor Rank 3", "Nature Resistance Rank 1", "Shadow Resistance Rank 1"}
	if len(names) != len(want) {
		t.Fatalf("abilities = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("abilities[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestControlsByPetActionFlag(t *testing.T) {
	s := snapshot.Snapshot{"pet": map[string]any{"exists": true, "abilities": []any{
		map[string]any{"name": "Knurren", "rank": "Rang 3"},
		map[string]any{"name": "Defensiv", "rank": "Begleiterhaltung", "petAction": true},
		map[string]any{"name": "Defensive", "rank": "Pet Stance"},
	}}}
	if got := abilityLabels(Inspect(s).Learned()); len(got) != 1 || got[0] != "Knurren Rang 3" {
		t.Errorf("abilities: %v", got)
	}
}
