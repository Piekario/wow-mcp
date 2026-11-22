// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package pet covers the character's pet (Hunter): happiness, loyalty, training points,
// diet, learned abilities and the food the character has in bags.
package pet

import (
	"fmt"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// NoPetNote says why there is no pet section: the character has no pet or the section is disabled.
const NoPetNote = "The dump has no pet section: the character has no pet (not a pet class) or the section is disabled in the addon (/mcp off pet; enable it with /mcp on pet)."

// Exists says whether the dump has a pet that is summoned (exists=true).
func Exists(s snapshot.Snapshot) bool { return s.Map("pet")["exists"] == true }

// happinessNames are the classic happiness states (1-3); the addon saves the number.
var happinessNames = map[int64]string{1: "Unhappy", 2: "Content", 3: "Happy"}

// View describes the pet. When there is no section or the pet is not summoned, it returns only an explanation.
func View(s snapshot.Snapshot) map[string]any {
	p := s.Map("pet")
	switch {
	case p == nil:
		return map[string]any{"exists": false, "note": NoPetNote}
	case p["exists"] != true:
		return map[string]any{"exists": false, "note": "The pet section is in the dump, but the pet was not summoned at dump time (dead, in the stable or dismissed)."}
	}
	out := map[string]any{"exists": true}
	for _, k := range []string{"name", "family", "level", "dead", "damagePercent", "loyaltyRate", "loyalty", "foodTypes",
		"happinessInfo", "loyaltyInfo", "trainingPointsInfo"} {
		if v, ok := p[k]; ok {
			out[k] = v
		}
	}
	if h, ok := snapshot.AsInt(p["happiness"]); ok {
		out["happiness"] = map[string]any{"value": h, "name": happinessNames[h]}
	}
	if pct, ok := xpPercent(p); ok {
		out["xpPercent"] = pct
	}
	if points := trainingPoints(p); points != nil {
		out["trainingPoints"] = points
	}
	if abilities := abilityNames(p); len(abilities) > 0 {
		out["abilities"] = abilities
	}
	if auto := autoCast(p); len(auto) > 0 {
		out["autoCast"] = auto
	}
	return out
}

func xpPercent(p map[string]any) (float64, bool) {
	xp, ok1 := snapshot.AsInt(p["xp"])
	xpMax, ok2 := snapshot.AsInt(p["xpMax"])
	if !ok1 || !ok2 || xpMax <= 0 {
		return 0, false
	}
	return float64(xp*1000/xpMax) / 10, true
}

// trainingPoints collects the training points: total, spent and free; nil when the addon did not save them.
func trainingPoints(p map[string]any) map[string]any {
	out := map[string]any{}
	for key, field := range map[string]string{"total": "trainingPointsTotal", "spent": "trainingPointsSpent", "free": "trainingPointsFree"} {
		if n, ok := snapshot.AsInt(p[field]); ok {
			out[key] = n
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isControl: stances and commands are not abilities, just pet controls. Addon 0.9.0 marks them petAction
// (the spellbook entry type, the same in every language); older dumps only have the English subtext
// "Pet Stance" / "Pet Command" in rank.
func isControl(a map[string]any) bool {
	if a["petAction"] == true {
		return true
	}
	rank, _ := a["rank"].(string)
	return rank == "Pet Stance" || rank == "Pet Command"
}

// abilityNames lists the pet's learned abilities: "Growl Rank 3" (without stances and commands).
func abilityNames(p map[string]any) []string {
	var out []string
	for _, raw := range snapshot.AsList(p["abilities"]) {
		a := snapshot.AsMap(raw)
		if isControl(a) {
			continue
		}
		name := fmt.Sprint(a["name"])
		if rank, ok := a["rank"].(string); ok && rank != "" {
			name += " " + rank
		}
		out = append(out, name)
	}
	return out
}

// autoCast says which abilities on the pet bar run automatically (true) or not (false).
func autoCast(p map[string]any) map[string]any {
	out := map[string]any{}
	for _, raw := range snapshot.AsList(p["actionBar"]) {
		a := snapshot.AsMap(raw)
		if on, ok := a["autoCast"].(bool); ok {
			out[fmt.Sprint(a["name"])] = on
		}
	}
	return out
}

// FoodInBags returns food from bags (the Food & Drink subclass; for old dumps a subtype containing "Food") that the character may have for the pet.
// The server has no table of which food is meat, fish etc., so matching to foodTypes is left to the reader.
func FoodInBags(s snapshot.Snapshot) []any {
	out := []any{}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			if itemkind.IsFood(item) {
				out = append(out, map[string]any{"name": item["name"], "count": snapshot.ItemCount(item), "id": item["id"]})
			}
		}
	}
	return out
}

// FoodNote explains why the server does not match food to the diet itself.
const FoodNote = "foodInBags is food from bags by item subtype; the server has no table of which food is meat, fish, cheese etc. " +
	"Match it to the pet's foodTypes by name, and when unsure, say so plainly."
