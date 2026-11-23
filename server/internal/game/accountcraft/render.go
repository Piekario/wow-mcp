// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package accountcraft

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// View lists the recipes of all characters against the stock of the whole account.
func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	report, err := analyse(db, q, fresh)
	if err != nil {
		return nil, err
	}
	return report.Render(fresh), nil
}

func (r Report) Render(fresh snapshot.Freshness) map[string]any {
	crafters := []any{}
	for _, c := range r.Crafters {
		profs := []any{}
		for _, b := range c.Books {
			profs = append(profs, b.render(fresh))
		}
		crafters = append(crafters, map[string]any{"character": c.Name, "professions": profs})
	}
	out := map[string]any{
		"crafters": crafters, "count": len(crafters),
		"note": "accountCanMake = how many can be crafted from the materials in the bags and banks of all characters combined; crafterCanMake = from the crafter's own materials only. borrowFrom suggests where to send missing materials from (mail between characters on the same realm and faction; the server sends nothing). Banks show the state from the last visits. Recipes under collapsed headers of the profession window are not included in the dump.",
	}
	if len(r.WithoutRecipes) > 0 {
		out["withoutRecipes"] = r.WithoutRecipes
		out["hint"] = "These characters have no saved recipes: log in on them, open the profession window (e.g. Leatherworking) and type /mcp reload."
	}
	if len(crafters) == 0 && len(r.WithoutRecipes) == r.Characters {
		out["note"] = "No saved recipes on the account. Open the profession window in game, then type /mcp reload. Each profession is saved separately."
	}
	return out
}

func (b CraftingBook) render(fresh snapshot.Freshness) map[string]any {
	views := make([]any, len(b.Recipes))
	for i, c := range b.Recipes {
		views[i] = c.render()
	}
	out := map[string]any{"name": b.Name, "rank": b.Rank, "maxRank": b.MaxRank, "recipes": views, "shown": len(views), "total": b.Total}
	if b.Truncated() {
		out["truncated"] = true
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(b.Captured))
	return out
}

func (c Craft) render() map[string]any {
	lines := []any{}
	for _, l := range c.Lines {
		holders := []any{}
		for _, h := range l.Holders {
			holders = append(holders, map[string]any{"character": h.Character, "count": h.Count})
		}
		lines = append(lines, map[string]any{"name": l.Name, "need": l.Need, "crafterHas": l.CrafterHas, "accountHas": l.AccountHas, "holders": holders})
	}
	v := map[string]any{"name": c.Label, "reagents": lines, "crafterCanMake": c.CrafterCanMake, "accountCanMake": c.AccountCanMake}
	if c.ItemID != nil {
		v["itemId"] = c.ItemID
	}
	if c.Difficulty != nil {
		v["difficulty"] = c.Difficulty
	}
	var missing, borrow []string
	for _, m := range c.Missing {
		missing = append(missing, fmt.Sprintf("%s: missing %d", m.Name, m.Missing))
	}
	for _, l := range c.Borrow {
		borrow = append(borrow, fmt.Sprintf("%s -> %s: %s x%d", l.From, l.To, l.Material, l.Count))
	}
	if len(missing) > 0 {
		v["missing"] = missing
	}
	if len(borrow) > 0 {
		v["borrowFrom"] = borrow
	}
	return v
}
