// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package accountcraft

import (
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// recipe keeps what the answer passes through as the dump has it: the name (which may be missing), the item and the
// difficulty.
type recipe struct {
	professions.Recipe
	label, itemID, difficulty any
}

type book struct {
	name          string
	rank, maxRank any
	captured      snapshot.Captured
	recipes       []recipe
}

func readStock(db *snapshot.Database) stock {
	out := stock{}
	for _, char := range db.CharacterNames() {
		for key, line := range items.StockOf(db.Characters[char], nil) {
			if out[key] == nil {
				out[key] = map[string]int64{}
			}
			out[key][char] = line.Total()
		}
	}
	return out
}

// readBooks returns the character's saved recipe books for the professions matching the fragment;
// saved is false when no profession window was ever saved.
func readBooks(s snapshot.Snapshot, profession string, fresh snapshot.Freshness) (books []book, saved bool) {
	report := professions.AnalyseRecipes(s, professions.RecipeQuery{Profession: profession}, fresh)
	for _, b := range report.Books {
		out := book{name: b.Name, rank: b.Rank, maxRank: b.MaxRank, captured: b.Captured}
		for _, r := range b.Recipes {
			raw := snapshot.AsMap(r.Raw)
			out.recipes = append(out.recipes, recipe{Recipe: r.Recipe, label: raw["name"], itemID: raw["itemId"], difficulty: raw["difficulty"]})
		}
		books = append(books, out)
	}
	return books, report.Saved
}
