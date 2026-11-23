// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package accountcraft

import (
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

func bag(items ...any) []any { return []any{map[string]any{"items": items}} }
func it(name string, n int64) map[string]any {
	return map[string]any{"name": name, "count": n}
}
func reagent(name string, need int64) map[string]any {
	return map[string]any{"name": name, "need": need}
}

func testDB() *snapshot.Database {
	lw := snapshot.Snapshot{
		"bags": bag(it("Light Leather", 2)),
		"recipes": map[string]any{"Leatherworking": map[string]any{"rank": int64(50), "recipes": []any{
			map[string]any{"name": "Gloves", "itemId": int64(1), "reagents": []any{reagent("Light Leather", 3), reagent("Coarse Thread", 2)}},
			map[string]any{"name": "Armor Kit", "reagents": []any{reagent("Light Leather", 1)}},
			map[string]any{"name": "Boots", "reagents": []any{reagent("Heavy Leather", 1)}},
			map[string]any{"name": "Growl"},
		}}},
	}
	alt := snapshot.Snapshot{
		"bags": bag(it("Light Leather", 5), it("Coarse Thread", 4)),
		"bank": map[string]any{"containers": []any{map[string]any{"items": []any{it("Light Leather", 1)}}}},
	}
	return &snapshot.Database{Characters: map[string]snapshot.Snapshot{"Lw": lw, "Alt": alt}}
}

func recipeByName(out map[string]any, name string) map[string]any {
	for _, c := range snapshot.AsList(out["crafters"]) {
		for _, p := range snapshot.AsList(snapshot.AsMap(c)["professions"]) {
			for _, r := range snapshot.AsList(snapshot.AsMap(p)["recipes"]) {
				if snapshot.AsMap(r)["name"] == name {
					return snapshot.AsMap(r)
				}
			}
		}
	}
	return nil
}

func TestView(t *testing.T) {
	out, err := View(testDB(), Query{}, fixtures.At(0))
	if err != nil {
		t.Fatal(err)
	}
	gloves := recipeByName(out, "Gloves")
	if gloves["accountCanMake"] != int64(2) || gloves["crafterCanMake"] != int64(0) {
		t.Errorf("Gloves: %v", gloves)
	}
	if b := gloves["borrowFrom"].([]string); len(b) != 2 || !strings.Contains(b[0], "Alt -> Lw: Light Leather x1") {
		t.Errorf("borrowFrom: %v", b)
	}
	boots := recipeByName(out, "Boots")
	if boots["accountCanMake"] != int64(0) || boots["missing"].([]string)[0] != "Heavy Leather: missing 1" {
		t.Errorf("Boots: %v", boots)
	}
	if recipeByName(out, "Growl") != nil {
		t.Error("a recipe without reagents must be skipped")
	}
	if out["withoutRecipes"].([]string)[0] != "Alt" {
		t.Errorf("withoutRecipes: %v", out["withoutRecipes"])
	}
	// Sorting: craftable first.
	recs := snapshot.AsList(snapshot.AsMap(snapshot.AsList(snapshot.AsMap(snapshot.AsList(out["crafters"])[0])["professions"])[0])["recipes"])
	if snapshot.AsMap(recs[len(recs)-1])["name"] != "Boots" {
		t.Errorf("order: %v", recs)
	}
}

func TestFilters(t *testing.T) {
	tests := []struct {
		name string
		q    Query
		want []string
	}{
		{"query by recipe", Query{Query: "armor"}, []string{"Armor Kit"}},
		{"query by material", Query{Query: "thread"}, []string{"Gloves"}},
		{"craftable", Query{Craftable: true}, []string{"Armor Kit", "Gloves"}},
		{"profession with no match", Query{Profession: "Tailor"}, nil},
		{"limit", Query{Limit: 1}, []string{"Armor Kit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := View(testDB(), tt.q, fixtures.At(0))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, c := range snapshot.AsList(out["crafters"]) {
				for _, p := range snapshot.AsList(snapshot.AsMap(c)["professions"]) {
					for _, r := range snapshot.AsList(snapshot.AsMap(p)["recipes"]) {
						got = append(got, snapshot.AsMap(r)["name"].(string))
					}
				}
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestErrorsAndNoRecipes(t *testing.T) {
	if _, err := View(testDB(), Query{Character: "Nope"}, fixtures.At(0)); err == nil {
		t.Error("expected an error for an unknown character")
	}
	if _, err := View(&snapshot.Database{}, Query{}, fixtures.At(0)); err == nil {
		t.Error("empty database = error")
	}
	empty := &snapshot.Database{Characters: map[string]snapshot.Snapshot{"A": {}}}
	out, _ := View(empty, Query{}, fixtures.At(0))
	if !strings.Contains(out["note"].(string), "/mcp reload") || out["count"] != 0 {
		t.Errorf("hint: %v", out)
	}
}

func TestAssessOnTypedResult(t *testing.T) {
	st := stock{"thread": {"Crafter": 1, "Alt": 5}, "dye": {"Alt": 1}}
	r := recipe{Recipe: professions.Recipe{Name: "Cloak", Reagents: []professions.Reagent{{Name: "Thread", Need: 2}, {Name: "Dye", Need: 3}}}, label: "Cloak"}
	c, ok := assess("Crafter", r, st)
	if !ok || c.AccountCanMake != 0 || c.CrafterCanMake != 0 {
		t.Fatalf("craft: %+v", c)
	}
	if len(c.Missing) != 1 || c.Missing[0] != (Shortfall{"Dye", 2}) {
		t.Errorf("missing: %+v", c.Missing)
	}
	if len(c.Borrow) != 1 || c.Borrow[0] != (Loan{"Alt", "Crafter", "Thread", 1}) {
		t.Errorf("borrow: %+v", c.Borrow)
	}
	if _, ok := assess("Crafter", recipe{}, st); ok {
		t.Error("a recipe without reagents is not assessed")
	}
}
