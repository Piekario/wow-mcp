// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package accountcraft answers who on the account can craft what, and from whose materials:
// for every known recipe it counts how many can be made from the materials
// lying in the bags and banks of ALL characters, and shows who holds what.
package accountcraft

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// defaultLimit is the maximum number of recipes per profession per character.
const defaultLimit = 40

// Query is the input of wow_account_crafting.
type Query struct {
	Character  string `json:"character,omitempty" jsonschema:"only this character's recipes; empty = all characters"`
	Profession string `json:"profession,omitempty" jsonschema:"profession name or a fragment of it; empty = all"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of a recipe or material name"`
	Craftable  bool   `json:"craftable,omitempty" jsonschema:"true = only recipes that can be crafted at least once from account materials"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max recipes per character profession; default 40"`
}

// stock is the account's material stock: name (lowercase) -> character -> count (bags + bank).
type stock map[string]map[string]int64

func buildStock(db *snapshot.Database) stock {
	out := stock{}
	for _, char := range db.CharacterNames() {
		for key, line := range items.BagBankStock(db.Characters[char], nil) {
			if out[key] == nil {
				out[key] = map[string]int64{}
			}
			out[key][char] = line.Total()
		}
	}
	return out
}

func (st stock) total(name string) int64 {
	var sum int64
	for _, n := range st[strings.ToLower(name)] {
		sum += n
	}
	return sum
}

// holders lists who holds a material, largest stock first.
func (st stock) holders(name string) []any {
	m := st[strings.ToLower(name)]
	chars := slices.Sorted(maps.Keys(m))
	sort.SliceStable(chars, func(i, j int) bool { return m[chars[i]] > m[chars[j]] })
	out := make([]any, 0, len(chars))
	for _, c := range chars {
		out = append(out, map[string]any{"character": c, "count": m[c]})
	}
	return out
}

// recipeMatches: the fragment matches the recipe name or one of its materials.
func recipeMatches(r map[string]any, query string) bool {
	if query == "" {
		return true
	}
	if n, _ := r["name"].(string); strings.Contains(strings.ToLower(n), query) {
		return true
	}
	for _, rr := range snapshot.AsList(r["reagents"]) {
		if n := snapshot.ReagentName(snapshot.AsMap(rr)); strings.Contains(strings.ToLower(n), query) {
			return true
		}
	}
	return false
}

type entry struct {
	view       map[string]any
	accountCan int64
}

// assess evaluates one recipe against the account stock; ok = false when it has no reagents (e.g. Beast Training).
func assess(crafter string, r map[string]any, st stock) (entry, bool) {
	reagents := snapshot.AsList(r["reagents"])
	if len(reagents) == 0 {
		return entry{}, false
	}
	accountCan, ownCan := int64(-1), int64(-1)
	var lines []any
	var missing, mail []string
	for _, rr := range reagents {
		re := snapshot.AsMap(rr)
		name := snapshot.ReagentName(re)
		need, _ := snapshot.AsInt(re["need"])
		if need <= 0 {
			need = 1
		}
		total := st.total(name)
		own := st[strings.ToLower(name)][crafter]
		line := map[string]any{"name": name, "need": need, "crafterHas": own, "accountHas": total, "holders": st.holders(name)}
		lines = append(lines, line)
		if c := total / need; accountCan < 0 || c < accountCan {
			accountCan = c
		}
		if c := own / need; ownCan < 0 || c < ownCan {
			ownCan = c
		}
		if total < need {
			missing = append(missing, fmt.Sprintf("%s: missing %d", name, need-total))
			continue
		}
		if own < need {
			for _, h := range st.holders(name) {
				hm := h.(map[string]any)
				if hm["character"] != crafter {
					mail = append(mail, fmt.Sprintf("%s -> %s: %s x%d", hm["character"], crafter, name, min(hm["count"].(int64), need-own)))
					break
				}
			}
		}
	}
	v := map[string]any{
		"name": r["name"], "reagents": lines,
		"crafterCanMake": ownCan, "accountCanMake": accountCan,
	}
	for _, k := range []string{"itemId", "difficulty"} {
		if val, ok := r[k]; ok {
			v[k] = val
		}
	}
	if len(missing) > 0 {
		v["missing"] = missing
	}
	if len(mail) > 0 {
		v["borrowFrom"] = mail
	}
	return entry{view: v, accountCan: accountCan}, true
}

// View lists the recipes of all characters against the stock of the whole account.
func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	if db == nil || len(db.Characters) == 0 {
		return nil, fmt.Errorf("no characters in the dump. %s", snapshot.AltsHint)
	}
	names := db.CharacterNames()
	if q.Character != "" {
		name, _, err := db.Lookup(q.Character)
		if err != nil {
			return nil, err
		}
		names = []string{name}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	st := buildStock(db)
	query, profQuery := strings.ToLower(q.Query), strings.ToLower(q.Profession)

	crafters := []any{}
	var withoutRecipes []string
	for _, name := range names {
		all := db.Characters[name].Map("recipes")
		if len(all) == 0 {
			withoutRecipes = append(withoutRecipes, name)
			continue
		}
		profs := []any{}
		for _, prof := range slices.Sorted(maps.Keys(all)) {
			if profQuery != "" && !strings.Contains(strings.ToLower(prof), profQuery) {
				continue
			}
			p := snapshot.AsMap(all[prof])
			var list []entry
			for _, raw := range snapshot.AsList(p["recipes"]) {
				r := snapshot.AsMap(raw)
				if !recipeMatches(r, query) {
					continue
				}
				e, ok := assess(name, r, st)
				if !ok || (q.Craftable && e.accountCan < 1) {
					continue
				}
				list = append(list, e)
			}
			sort.SliceStable(list, func(i, j int) bool { return list[i].accountCan > list[j].accountCan })
			shown := list
			if len(shown) > limit {
				shown = shown[:limit]
			}
			views := make([]any, len(shown))
			for i, e := range shown {
				views[i] = e.view
			}
			pe := map[string]any{"name": prof, "rank": p["rank"], "maxRank": p["maxRank"], "recipes": views, "shown": len(views), "total": len(list)}
			if len(list) > len(shown) {
				pe["truncated"] = true
			}
			snapshot.Merge(pe, fresh.CapturedMeta(p))
			profs = append(profs, pe)
		}
		if len(profs) > 0 {
			crafters = append(crafters, map[string]any{"character": name, "professions": profs})
		}
	}
	out := map[string]any{
		"crafters": crafters, "count": len(crafters),
		"note": "accountCanMake = how many can be crafted from the materials in the bags and banks of all characters combined; crafterCanMake = from the crafter's own materials only. borrowFrom suggests where to send missing materials from (mail between characters on the same realm and faction; the server sends nothing). Banks show the state from the last visits. Recipes under collapsed headers of the profession window are not included in the dump.",
	}
	if len(withoutRecipes) > 0 {
		out["withoutRecipes"] = withoutRecipes
		out["hint"] = "These characters have no saved recipes: log in on them, open the profession window (e.g. Leatherworking) and type /mcp reload."
	}
	if len(crafters) == 0 && len(withoutRecipes) == len(names) {
		out["note"] = "No saved recipes on the account. Open the profession window in game, then type /mcp reload. Each profession is saved separately."
	}
	return out, nil
}
