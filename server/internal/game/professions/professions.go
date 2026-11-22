// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package professions covers the character's professions and the recipes from profession windows
// (including the Hunter's Beast Training).
package professions

import (
	"fmt"
	"maps"
	"slices"
	"strings"

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

// SkillOf returns the skill line ID of a professions entry: skillID from the dump, or for an old dump the ID
// of the profession with that English name. false when neither is known.
func SkillOf(p map[string]any) (int64, bool) {
	if id, ok := snapshot.AsInt(p["skillID"]); ok {
		return id, true
	}
	name, _ := p["name"].(string)
	for id, english := range EnglishNames {
		if strings.EqualFold(english, name) {
			return id, true
		}
	}
	return 0, false
}

// isProfession tells professions apart from weapon, language and armor skills in the professions list:
// first by the skill line ID, then by the group header (English on old dumps).
func isProfession(p map[string]any) bool {
	if id, ok := snapshot.AsInt(p["skillID"]); ok {
		_, known := EnglishNames[id]
		return known
	}
	group, _ := p["group"].(string)
	if group == "" {
		return true
	}
	g := strings.ToLower(group)
	return strings.Contains(g, "profession") || strings.Contains(g, "secondary")
}

// Summary writes professions briefly: "Skinning 96/150".
func Summary(s snapshot.Snapshot) []string {
	var out []string
	for _, raw := range s.List("professions") {
		p := snapshot.AsMap(raw)
		if !isProfession(p) {
			continue
		}
		rank, _ := snapshot.AsInt(p["rank"])
		maxRank, _ := snapshot.AsInt(p["maxRank"])
		out = append(out, fmt.Sprintf("%v %d/%d", p["name"], rank, maxRank))
	}
	return out
}

// RecipeQuery holds the recipe filters (wow_recipes).
type RecipeQuery struct {
	Character  string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Profession string `json:"profession,omitempty" jsonschema:"profession name or a fragment of it, e.g. Leatherworking, Beast Training; empty = all saved"`
	Query      string `json:"query,omitempty" jsonschema:"fragment of a recipe name"`
	SkillUp    bool   `json:"skillUp,omitempty" jsonschema:"true = only recipes that still give skill points (orange, yellow, green)"`
	Craftable  bool   `json:"craftable,omitempty" jsonschema:"true = only what can be crafted right now from materials in bags"`
}

var difficultyInfo = map[string]string{
	"optimal": "orange: always gives a skill point",
	"medium":  "yellow: usually gives a skill point",
	"easy":    "green: sometimes gives a skill point",
	"trivial": "gray: gives no skill points",
}

// givesSkillUp: a recipe of this color can still raise the skill.
func givesSkillUp(difficulty string) bool {
	return difficulty == "optimal" || difficulty == "medium" || difficulty == "easy"
}

// bankCounts sums bank items by name to show the materials stored in the bank.
func bankCounts(s snapshot.Snapshot) map[string]int64 {
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

// Recipes describes the recipes of every saved profession: difficulty color, missing
// materials, materials in the bank, cooldowns and what can be crafted right now for skill points.
func Recipes(s snapshot.Snapshot, q RecipeQuery, fresh snapshot.Freshness) map[string]any {
	all := s.Map("recipes")
	if len(all) == 0 {
		return map[string]any{"note": "No saved recipes. Open the profession window in game (e.g. Leatherworking or a Hunter's Beast Training), then type /mcp reload. Each profession is saved separately."}
	}
	inBank := bankCounts(s)
	professions := []any{}
	for _, name := range slices.Sorted(maps.Keys(all)) {
		if q.Profession != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(q.Profession)) {
			continue
		}
		professions = append(professions, professionRecipes(name, snapshot.AsMap(all[name]), q, inBank, fresh))
	}
	out := map[string]any{"professions": professions}
	if pet := s.Map("pet"); pet != nil {
		if tp, ok := pet["trainingPointsFree"]; ok {
			out["petTrainingPointsFree"] = tp
		}
	}
	out["note"] = "Recipes under collapsed headers in the profession window are not included in the dump. canCraft = how many can be crafted from the materials in bags; inBank = how much of a given material is in the bank (state from the last visit)."
	return out
}

// professionRecipes describes the recipes of one profession.
func professionRecipes(name string, p map[string]any, q RecipeQuery, inBank map[string]int64, fresh snapshot.Freshness) map[string]any {
	query := strings.ToLower(q.Query)
	counts := map[string]int{}
	recipes := []any{}
	var skillUpNow []string
	for _, raw := range snapshot.AsList(p["recipes"]) {
		r := snapshot.AsMap(raw)
		diff, _ := r["difficulty"].(string)
		counts[diff]++
		skillUp := givesSkillUp(diff)
		craft, _ := snapshot.AsInt(r["canCraft"])
		rn, _ := r["name"].(string)
		if query != "" && !strings.Contains(strings.ToLower(rn), query) {
			continue
		}
		if q.SkillUp && !skillUp {
			continue
		}
		if q.Craftable && craft <= 0 {
			continue
		}
		recipes = append(recipes, recipeEntry(r, diff, inBank, fresh))
		if skillUp && craft > 0 && (diff == "optimal" || diff == "medium") {
			skillUpNow = append(skillUpNow, fmt.Sprintf("%s (x%d)", rn, craft))
		}
	}
	entry := map[string]any{
		"name": name, "rank": p["rank"], "maxRank": p["maxRank"],
		"recipes": recipes, "shown": len(recipes), "byDifficulty": counts,
	}
	if len(skillUpNow) > 0 {
		entry["skillUpCraftableNow"] = skillUpNow
	}
	snapshot.Merge(entry, fresh.CapturedMeta(p))
	return entry
}

// recipeEntry is a recipe with a difficulty description, missing materials and cooldown.
// Note: it adds inBank to the materials in the dump itself (shared maps).
func recipeEntry(r map[string]any, diff string, inBank map[string]int64, fresh snapshot.Freshness) map[string]any {
	e := map[string]any{}
	for k, v := range r {
		e[k] = v
	}
	if info, ok := difficultyInfo[diff]; ok {
		e["difficultyInfo"] = info
	}
	var missing []string
	for _, rr := range snapshot.AsList(r["reagents"]) {
		re := snapshot.AsMap(rr)
		need, _ := snapshot.AsInt(re["need"])
		have, _ := snapshot.AsInt(re["have"])
		rname := snapshot.ReagentName(re)
		if b := inBank[rname]; b > 0 {
			re["inBank"] = b
		}
		if have < need {
			missing = append(missing, fmt.Sprintf("%s %d", rname, need-have))
		}
	}
	if len(missing) > 0 {
		e["missingForOne"] = missing
	}
	if at, ok := snapshot.AsInt(r["readyAt"]); ok {
		if left, ready := fresh.Until(at); ready {
			e["cooldown"] = "ready"
		} else {
			e["cooldown"] = snapshot.FormatAge(left)
		}
	}
	return e
}

// KnownReagents returns the materials used by known recipes from the saved profession windows:
// material name (lowercase) -> "Recipe (Profession)", the first one encountered.
func KnownReagents(s snapshot.Snapshot) map[string]string {
	out := map[string]string{}
	all := s.Map("recipes")
	for _, prof := range slices.Sorted(maps.Keys(all)) {
		for _, raw := range snapshot.AsList(snapshot.AsMap(all[prof])["recipes"]) {
			r := snapshot.AsMap(raw)
			for _, rr := range snapshot.AsList(r["reagents"]) {
				name := snapshot.ReagentName(snapshot.AsMap(rr))
				key := strings.ToLower(name)
				if _, seen := out[key]; name != "" && !seen {
					out[key] = fmt.Sprintf("%v (%s)", r["name"], prof)
				}
			}
		}
	}
	return out
}

// HasSkill says whether the character has a profession with this skill line ID (by skillID, or for an old dump
// by the profession's English name).
func HasSkill(s snapshot.Snapshot, skill int64) bool {
	for _, raw := range s.List("professions") {
		if id, ok := SkillOf(snapshot.AsMap(raw)); ok && id == skill {
			return true
		}
	}
	return false
}
