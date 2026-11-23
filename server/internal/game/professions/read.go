// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package professions

import (
	"fmt"
	"maps"
	"slices"

	"wowmcp/internal/snapshot"
)

func readProfession(m map[string]any) Profession {
	var p Profession
	if name, ok := m["name"].(string); ok {
		p.Name = name
	} else {
		p.Name = fmt.Sprint(m["name"])
	}
	p.SkillID, p.HasSkillID = snapshot.AsInt(m["skillID"])
	p.Group, _ = m["group"].(string)
	p.Rank, _ = snapshot.AsInt(m["rank"])
	p.MaxRank, _ = snapshot.AsInt(m["maxRank"])
	return p
}

func readProfessions(s snapshot.Snapshot) []Profession {
	var out []Profession
	for _, raw := range s.List("professions") {
		out = append(out, readProfession(snapshot.AsMap(raw)))
	}
	return out
}

// SkillOf returns the skill line ID of a professions entry of the dump.
func SkillOf(p map[string]any) (int64, bool) { return readProfession(p).SkillLine() }

func readReagent(raw any) Reagent {
	m := snapshot.AsMap(raw)
	re := Reagent{Name: snapshot.ReagentName(m), Raw: raw}
	re.Need, _ = snapshot.AsInt(m["need"])
	re.Have, _ = snapshot.AsInt(m["have"])
	return re
}

func readRecipe(raw any) Recipe {
	m := snapshot.AsMap(raw)
	r := Recipe{Raw: raw}
	r.Name, _ = m["name"].(string)
	difficulty, _ := m["difficulty"].(string)
	r.Difficulty = Difficulty(difficulty)
	r.CanCraft, _ = snapshot.AsInt(m["canCraft"])
	r.ReadyAt, r.HasReadyAt = snapshot.AsInt(m["readyAt"])
	for _, re := range snapshot.AsList(m["reagents"]) {
		r.Reagents = append(r.Reagents, readReagent(re))
	}
	return r
}

// readRecipeBooks returns the saved recipe books ordered by profession name.
func readRecipeBooks(s snapshot.Snapshot) []RecipeBook {
	all := s.Map("recipes")
	var books []RecipeBook
	for _, name := range slices.Sorted(maps.Keys(all)) {
		m := snapshot.AsMap(all[name])
		book := RecipeBook{Name: name, Rank: m["rank"], MaxRank: m["maxRank"], Captured: snapshot.CapturedOf(m)}
		for _, raw := range snapshot.AsList(m["recipes"]) {
			book.Recipes = append(book.Recipes, readRecipe(raw))
		}
		books = append(books, book)
	}
	return books
}

func readBankStock(s snapshot.Snapshot) map[string]int64 {
	out := map[string]int64{}
	for _, rawC := range s.BankContainers() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(rawC)["items"]) {
			item := snapshot.AsMap(raw)
			if name, _ := item["name"].(string); name != "" {
				out[name] += snapshot.ItemCount(item)
			}
		}
	}
	return out
}

func readPetTrainingPointsFree(s snapshot.Snapshot) (any, bool) {
	v, ok := s.Map("pet")["trainingPointsFree"]
	return v, ok
}
