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

func (st stock) total(name string) int64 {
	var sum int64
	for _, n := range st[strings.ToLower(name)] {
		sum += n
	}
	return sum
}

type Holder struct {
	Character string
	Count     int64
}

// holders lists who holds a material, largest stock first.
func (st stock) holders(name string) []Holder {
	m := st[strings.ToLower(name)]
	chars := slices.Sorted(maps.Keys(m))
	sort.SliceStable(chars, func(i, j int) bool { return m[chars[i]] > m[chars[j]] })
	out := make([]Holder, 0, len(chars))
	for _, c := range chars {
		out = append(out, Holder{c, m[c]})
	}
	return out
}

// ReagentLine is one material of a recipe against the crafter's own stock and the whole account's.
type ReagentLine struct {
	Name                         string
	Need, CrafterHas, AccountHas int64
	Holders                      []Holder
}

type Shortfall struct {
	Name    string
	Missing int64
}

// Loan is a parcel that would let the crafter make the recipe: Count of Material from From to To.
type Loan struct {
	From, To, Material string
	Count              int64
}

// Craft is one recipe judged against the account's materials. Label, ItemID and Difficulty are the dump's values
// (nil when the dump has none).
type Craft struct {
	Label, ItemID, Difficulty      any
	Lines                          []ReagentLine
	CrafterCanMake, AccountCanMake int64
	Missing                        []Shortfall
	Borrow                         []Loan
}

type CraftingBook struct {
	Name          string
	Rank, MaxRank any
	Recipes       []Craft
	Total         int
	Captured      snapshot.Captured
}

func (b CraftingBook) Truncated() bool { return b.Total > len(b.Recipes) }

type Crafter struct {
	Name  string
	Books []CraftingBook
}

// Report is who can craft what. WithoutRecipes are the characters that never saved a profession window.
type Report struct {
	Crafters       []Crafter
	WithoutRecipes []string
	Characters     int
}

// matches: the fragment matches the recipe name or one of its materials.
func (r recipe) matches(query string) bool {
	if query == "" || strings.Contains(strings.ToLower(r.Name), query) {
		return true
	}
	for _, re := range r.Reagents {
		if strings.Contains(strings.ToLower(re.Name), query) {
			return true
		}
	}
	return false
}

// assess evaluates one recipe against the account stock; ok = false when it has no reagents (e.g. Beast Training).
func assess(crafter string, r recipe, st stock) (Craft, bool) {
	if len(r.Reagents) == 0 {
		return Craft{}, false
	}
	c := Craft{Label: r.label, ItemID: r.itemID, Difficulty: r.difficulty, AccountCanMake: -1, CrafterCanMake: -1}
	for _, re := range r.Reagents {
		need := max(re.Need, 1)
		total := st.total(re.Name)
		own := st[strings.ToLower(re.Name)][crafter]
		holders := st.holders(re.Name)
		c.Lines = append(c.Lines, ReagentLine{Name: re.Name, Need: need, CrafterHas: own, AccountHas: total, Holders: holders})
		if n := total / need; c.AccountCanMake < 0 || n < c.AccountCanMake {
			c.AccountCanMake = n
		}
		if n := own / need; c.CrafterCanMake < 0 || n < c.CrafterCanMake {
			c.CrafterCanMake = n
		}
		if total < need {
			c.Missing = append(c.Missing, Shortfall{re.Name, need - total})
		} else if own < need {
			for _, h := range holders {
				if h.Character != crafter {
					c.Borrow = append(c.Borrow, Loan{h.Character, crafter, re.Name, min(h.Count, need-own)})
					break
				}
			}
		}
	}
	return c, true
}

func analyse(db *snapshot.Database, q Query, fresh snapshot.Freshness) (Report, error) {
	if db == nil || len(db.Characters) == 0 {
		return Report{}, fmt.Errorf("no characters in the dump. %s", snapshot.AltsHint)
	}
	names := db.CharacterNames()
	if q.Character != "" {
		name, _, err := db.Lookup(q.Character)
		if err != nil {
			return Report{}, err
		}
		names = []string{name}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	st := readStock(db)
	query := strings.ToLower(q.Query)

	report := Report{Characters: len(names)}
	for _, name := range names {
		books, saved := readBooks(db.Characters[name], q.Profession, fresh)
		if !saved {
			report.WithoutRecipes = append(report.WithoutRecipes, name)
			continue
		}
		var crafted []CraftingBook
		for _, b := range books {
			crafted = append(crafted, craftingBook(name, b, st, query, q.Craftable, limit))
		}
		if len(crafted) > 0 {
			report.Crafters = append(report.Crafters, Crafter{name, crafted})
		}
	}
	return report, nil
}

func craftingBook(crafter string, b book, st stock, query string, craftableOnly bool, limit int) CraftingBook {
	var list []Craft
	for _, r := range b.recipes {
		if !r.matches(query) {
			continue
		}
		c, ok := assess(crafter, r, st)
		if !ok || (craftableOnly && c.AccountCanMake < 1) {
			continue
		}
		list = append(list, c)
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].AccountCanMake > list[j].AccountCanMake })
	out := CraftingBook{Name: b.name, Rank: b.rank, MaxRank: b.maxRank, Total: len(list), Captured: b.captured}
	out.Recipes = list[:min(len(list), limit)]
	return out
}
