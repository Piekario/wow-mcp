// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package professions covers the character's professions and the recipes from profession windows
// (including the Hunter's Beast Training).
package professions

import (
	"strings"
	"time"

	"wowmcp/internal/snapshot"
)

// Skill line IDs of the professions: the same on every client and since the classic game (the addon saves them as
// skillID, addon 0.9.0; weapon skills had them before).
const (
	SkillFirstAid       int64 = 129
	SkillBlacksmithing  int64 = 164
	SkillLeatherworking int64 = 165
	SkillAlchemy        int64 = 171
	SkillHerbalism      int64 = 182
	SkillCooking        int64 = 185
	SkillMining         int64 = 186
	SkillTailoring      int64 = 197
	SkillEngineering    int64 = 202
	SkillEnchanting     int64 = 333
	SkillFishing        int64 = 356
	SkillSkinning       int64 = 393
)

// EnglishNames is the English name of each profession skill: the fallback for dumps without skillID.
var EnglishNames = map[int64]string{
	SkillFirstAid: "First Aid", SkillBlacksmithing: "Blacksmithing", SkillLeatherworking: "Leatherworking",
	SkillAlchemy: "Alchemy", SkillHerbalism: "Herbalism", SkillCooking: "Cooking", SkillMining: "Mining",
	SkillTailoring: "Tailoring", SkillEngineering: "Engineering", SkillEnchanting: "Enchanting",
	SkillFishing: "Fishing", SkillSkinning: "Skinning",
}

// Profession is an entry of the professions section: a profession, or a weapon, language or armor skill.
type Profession struct {
	Name          string
	SkillID       int64
	HasSkillID    bool
	Group         string
	Rank, MaxRank int64
}

// SkillLine is the skill line ID of the entry: skillID from the dump, or for an old dump the ID of the profession
// with that English name. false when neither is known.
func (p Profession) SkillLine() (int64, bool) {
	if p.HasSkillID {
		return p.SkillID, true
	}
	for id, english := range EnglishNames {
		if strings.EqualFold(english, p.Name) {
			return id, true
		}
	}
	return 0, false
}

// IsProfession tells professions apart from weapon, language and armor skills in the professions list:
// first by the skill line ID, then by the group header (English on old dumps).
func (p Profession) IsProfession() bool {
	if p.HasSkillID {
		_, known := EnglishNames[p.SkillID]
		return known
	}
	if p.Group == "" {
		return true
	}
	g := strings.ToLower(p.Group)
	return strings.Contains(g, "profession") || strings.Contains(g, "secondary")
}

// Known returns the character's professions in dump order, without weapon, language and armor skills.
func Known(s snapshot.Snapshot) []Profession {
	var out []Profession
	for _, p := range readProfessions(s) {
		if p.IsProfession() {
			out = append(out, p)
		}
	}
	return out
}

// HasSkill says whether the character has a profession with this skill line ID (by skillID, or for an old dump
// by the profession's English name).
func HasSkill(s snapshot.Snapshot, skill int64) bool {
	for _, p := range readProfessions(s) {
		if id, ok := p.SkillLine(); ok && id == skill {
			return true
		}
	}
	return false
}

// RecipeQuery holds the recipe filters (wow_recipes).
type RecipeQuery struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Profession string `json:"profession,omitempty" jsonschema:"profession name or a fragment of it, e.g. Leatherworking, Beast Training; empty = all saved"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of a recipe name"`
	SkillUp    bool   `json:"skillUp,omitempty" jsonschema:"true = only recipes that still give skill points (orange, yellow, green)"`
	Craftable  bool   `json:"craftable,omitempty" jsonschema:"true = only what can be crafted right now from materials in bags"`
}

// Difficulty is the color class of a recipe for the character's skill.
type Difficulty string

const (
	Optimal Difficulty = "optimal"
	Medium  Difficulty = "medium"
	Easy    Difficulty = "easy"
	Trivial Difficulty = "trivial"
)

// GivesSkillUp: a recipe of this color can still raise the skill.
func (d Difficulty) GivesSkillUp() bool { return d == Optimal || d == Medium || d == Easy }

// LikelySkillUp: orange and yellow recipes give a skill point always or usually.
func (d Difficulty) LikelySkillUp() bool { return d == Optimal || d == Medium }

type Reagent struct {
	Name       string
	Need, Have int64
	Raw        any
}

// Recipe keeps Raw, the recipe's dump node, because the answer passes through fields (trainingPoints, levelRequired...)
// that no rule uses.
type Recipe struct {
	Name       string
	Difficulty Difficulty
	CanCraft   int64
	ReadyAt    int64
	HasReadyAt bool
	Reagents   []Reagent
	Raw        any
}

// UnnamedBook is the name of a recipe window for which the client gave no profession name (the addon uses the same text).
const UnnamedBook = "Unnamed profession"

// RecipeBook is the recipes saved from one profession window. Rank and MaxRank stay as the dump has them.
type RecipeBook struct {
	Name          string
	Rank, MaxRank any
	Recipes       []Recipe
	Captured      snapshot.Captured
}

type ReagentStatus struct {
	Reagent
	InBank int64
}

type Shortfall struct {
	Name    string
	Missing int64
}

// Cooldown is the state of a recipe's cooldown; Known is false for a recipe without one.
type Cooldown struct {
	Known bool
	Ready bool
	Left  time.Duration
}

type RecipeStatus struct {
	Recipe
	Stock    []ReagentStatus
	Missing  []Shortfall
	Cooldown Cooldown
}

// Craftable is a recipe that raises the skill and can be crafted now.
type Craftable struct {
	Name  string
	Times int64
}

// BookReport is one profession's recipes after the filters; ByDifficulty counts all of the book's recipes.
type BookReport struct {
	Name          string
	Rank, MaxRank any
	Recipes       []RecipeStatus
	ByDifficulty  map[Difficulty]int
	SkillUpNow    []Craftable
	Captured      snapshot.Captured
}

// RecipeReport is the saved recipe books judged against the query, the bank and the clock.
// Saved is false when no profession window was ever saved.
type RecipeReport struct {
	Saved        bool
	Books        []BookReport
	PetPoints    any
	HasPetPoints bool
}

// AnalyseRecipes judges the recipes of every saved profession: difficulty, missing materials, materials in the
// bank, cooldowns and what can be crafted right now for skill points.
func AnalyseRecipes(s snapshot.Snapshot, q RecipeQuery, fresh snapshot.Freshness) RecipeReport {
	books := readRecipeBooks(s)
	if len(books) == 0 {
		return RecipeReport{}
	}
	inBank := readBankStock(s)
	report := RecipeReport{Saved: true}
	for _, book := range books {
		if q.Profession != "" && !strings.Contains(strings.ToLower(book.Name), strings.ToLower(q.Profession)) {
			continue
		}
		report.Books = append(report.Books, judgeBook(book, q, inBank, fresh))
	}
	report.PetPoints, report.HasPetPoints = readPetTrainingPointsFree(s)
	return report
}

func judgeBook(book RecipeBook, q RecipeQuery, inBank map[string]int64, fresh snapshot.Freshness) BookReport {
	query := strings.ToLower(q.Query)
	report := BookReport{Name: book.Name, Rank: book.Rank, MaxRank: book.MaxRank, ByDifficulty: map[Difficulty]int{}, Captured: book.Captured}
	for _, r := range book.Recipes {
		report.ByDifficulty[r.Difficulty]++
		if query != "" && !strings.Contains(strings.ToLower(r.Name), query) {
			continue
		}
		if q.SkillUp && !r.Difficulty.GivesSkillUp() {
			continue
		}
		if q.Craftable && r.CanCraft <= 0 {
			continue
		}
		report.Recipes = append(report.Recipes, judgeRecipe(r, inBank, fresh))
		if r.Difficulty.LikelySkillUp() && r.CanCraft > 0 {
			report.SkillUpNow = append(report.SkillUpNow, Craftable{r.Name, r.CanCraft})
		}
	}
	return report
}

func judgeRecipe(r Recipe, inBank map[string]int64, fresh snapshot.Freshness) RecipeStatus {
	status := RecipeStatus{Recipe: r}
	for _, re := range r.Reagents {
		status.Stock = append(status.Stock, ReagentStatus{Reagent: re, InBank: inBank[re.Name]})
		if re.Have < re.Need {
			status.Missing = append(status.Missing, Shortfall{re.Name, re.Need - re.Have})
		}
	}
	if r.HasReadyAt {
		left, ready := fresh.Until(r.ReadyAt)
		status.Cooldown = Cooldown{Known: true, Ready: ready, Left: left}
	}
	return status
}

// KnownReagents returns the materials used by known recipes from the saved profession windows:
// material name (lowercase) -> "Recipe (Profession)", the first one encountered.
func KnownReagents(s snapshot.Snapshot) map[string]string {
	out := map[string]string{}
	for _, book := range readRecipeBooks(s) {
		for _, r := range book.Recipes {
			for _, re := range r.Reagents {
				key := strings.ToLower(re.Name)
				if _, seen := out[key]; re.Name != "" && !seen {
					out[key] = r.Name + " (" + book.Name + ")"
				}
			}
		}
	}
	return out
}
