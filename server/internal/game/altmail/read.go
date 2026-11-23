// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package altmail

import (
	"fmt"
	"slices"
	"strings"

	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// sendable is the character's materials that can be mailed: a bound (Soulbound) or quest item cannot.
func sendable(s snapshot.Snapshot) map[string]*items.ItemStock {
	return items.StockOf(s, func(it items.Item) bool { return it.Tradable() })
}

func readRecipient(name string, s snapshot.Snapshot, fresh snapshot.Freshness) recipient {
	books := professions.AnalyseRecipes(s, professions.RecipeQuery{}, fresh).Books
	return recipient{name: name, uses: reagentUses(books), weak: weakProfessions(s, books), held: sendable(s)}
}

func reagentUses(books []professions.BookReport) map[string][]string {
	out := map[string][]string{}
	for _, book := range books {
		for _, r := range book.Recipes {
			for _, re := range r.Reagents {
				key := strings.ToLower(re.Name)
				label := fmt.Sprintf("%s (%s)", r.Name, book.Name)
				if re.Name != "" && !slices.Contains(out[key], label) {
					out[key] = append(out[key], label)
				}
			}
		}
	}
	return out
}

// weakProfessions is the character's professions for which no recipes are saved.
func weakProfessions(s snapshot.Snapshot, books []professions.BookReport) []weakProfession {
	var out []weakProfession
	for _, p := range professions.Known(s) {
		skill, ok := p.SkillLine()
		if _, known := professionKinds[skill]; !ok || !known {
			continue
		}
		if slices.ContainsFunc(books, func(b professions.BookReport) bool { return b.Name == p.Name }) {
			continue
		}
		out = append(out, weakProfession{p.Name, skill})
	}
	return out
}
