// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package professions

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

var difficultyInfo = map[Difficulty]string{
	Optimal: "orange: always gives a skill point",
	Medium:  "yellow: usually gives a skill point",
	Easy:    "green: sometimes gives a skill point",
	Trivial: "gray: gives no skill points",
}

// Summary writes professions briefly: "Skinning 96/150".
func Summary(s snapshot.Snapshot) []string {
	var out []string
	for _, p := range Known(s) {
		out = append(out, p.Label())
	}
	return out
}

// Label writes a profession briefly: "Skinning 96/150".
func (p Profession) Label() string { return fmt.Sprintf("%s %d/%d", p.Name, p.Rank, p.MaxRank) }

// Recipes describes the recipes of every saved profession for wow_recipes.
func Recipes(s snapshot.Snapshot, q RecipeQuery, fresh snapshot.Freshness) map[string]any {
	return AnalyseRecipes(s, q, fresh).Render(fresh)
}

func (r RecipeReport) Render(fresh snapshot.Freshness) map[string]any {
	if !r.Saved {
		return map[string]any{"note": "No saved recipes. Open the profession window in game (e.g. Leatherworking or a Hunter's Beast Training), then type /mcp reload. Each profession is saved separately."}
	}
	professions := []any{}
	for _, book := range r.Books {
		professions = append(professions, renderBook(book, fresh))
	}
	out := map[string]any{"professions": professions}
	if r.HasPetPoints {
		out["petTrainingPointsFree"] = r.PetPoints
	}
	out["note"] = "Recipes under collapsed headers in the profession window are not included in the dump. canCraft = how many can be crafted from the materials in bags; inBank = how much of a given material is in the bank (state from the last visit)."
	return out
}

func renderBook(b BookReport, fresh snapshot.Freshness) map[string]any {
	recipes := []any{}
	for _, r := range b.Recipes {
		recipes = append(recipes, renderRecipe(r))
	}
	counts := map[string]int{}
	for d, n := range b.ByDifficulty {
		counts[string(d)] = n
	}
	entry := map[string]any{
		"name": b.Name, "rank": b.Rank, "maxRank": b.MaxRank,
		"recipes": recipes, "shown": len(recipes), "byDifficulty": counts,
	}
	if len(b.SkillUpNow) > 0 {
		labels := make([]string, len(b.SkillUpNow))
		for i, c := range b.SkillUpNow {
			labels[i] = fmt.Sprintf("%s (x%d)", c.Name, c.Times)
		}
		entry["skillUpCraftableNow"] = labels
	}
	snapshot.Merge(entry, fresh.CapturedMetaOf(b.Captured))
	return entry
}

// renderRecipe passes the recipe's own fields through and adds what the rules found.
func renderRecipe(r RecipeStatus) map[string]any {
	e := map[string]any{}
	for k, v := range snapshot.AsMap(r.Raw) {
		e[k] = v
	}
	if info, ok := difficultyInfo[r.Difficulty]; ok {
		e["difficultyInfo"] = info
	}
	if reagents, ok := e["reagents"].([]any); ok {
		e["reagents"] = renderReagents(reagents, r.Stock)
	}
	if len(r.Missing) > 0 {
		missing := make([]string, len(r.Missing))
		for i, m := range r.Missing {
			missing[i] = fmt.Sprintf("%s %d", m.Name, m.Missing)
		}
		e["missingForOne"] = missing
	}
	if r.Cooldown.Known {
		if r.Cooldown.Ready {
			e["cooldown"] = "ready"
		} else {
			e["cooldown"] = snapshot.FormatAge(r.Cooldown.Left)
		}
	}
	return e
}

func renderReagents(raw []any, stock []ReagentStatus) []any {
	out := make([]any, len(raw))
	for i, item := range raw {
		out[i] = item
		if m := snapshot.AsMap(item); m != nil && stock[i].InBank > 0 {
			withStock := map[string]any{"inBank": stock[i].InBank}
			for k, v := range m {
				withStock[k] = v
			}
			out[i] = withStock
		}
	}
	return out
}
