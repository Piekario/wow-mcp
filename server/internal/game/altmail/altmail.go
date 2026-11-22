// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package altmail suggests what to mail between an account's characters: materials
// lying in one character's bags and bank that are ingredients of another character's recipes.
// The server only reads data and sends nothing; the player sends the parcel in game.
package altmail

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_alt_mail.
type Query struct {
	From  string `json:"from,omitempty" jsonschema:"only materials of this character (sender); empty = all characters"`
	To    string `json:"to,omitempty" jsonschema:"only for this character (recipient); empty = all characters"`
	Query string `json:"query,omitempty" jsonschema:"material name fragment"`
}

// The kind of evidence that the recipient needs the material.
const (
	EvidenceRecipe     = "recipe"     // the material is an ingredient of a known recipe (the recipes section)
	EvidenceProfession = "profession" // only a match of the item class to the profession: weaker evidence
)

// weakAssumption is the text on every item with weak evidence (evidence=profession).
const weakAssumption = "Assumption, not fact: the material is on the list only because its kind (Trade Goods subtype) is usually associated with the recipient's profession, " +
	"and its recipes are not saved in the dump. Check in game whether the recipient really uses it."

// professionKinds says which Trade Goods kind (itemkind.TradeGoodsKind) a profession consumes, by skill line ID.
// Only the obvious links from classic WoW (leather - Leatherworking, cloth - Tailoring, metal and stone -
// Blacksmithing, Engineering, herbs - Alchemy, enchanting ingredients, meat and fish - Cooking). We skip everything
// else so as not to invent mechanics.
var professionKinds = map[int64][]string{
	professions.SkillLeatherworking: {"Leather"},
	professions.SkillTailoring:      {"Cloth"},
	professions.SkillBlacksmithing:  {"Metal & Stone"},
	professions.SkillEngineering:    {"Metal & Stone"},
	professions.SkillAlchemy:        {"Herb"},
	professions.SkillEnchanting:     {"Enchanting"},
	professions.SkillCooking:        {"Cooking"},
}

// weakProfession is a recipient's profession without saved recipes: its name in the dump and its skill line ID.
type weakProfession struct {
	name  string
	skill int64
}

// stack is a character's material summed by name (only what can be sent).
type stack struct {
	items.StockLine
}

func (st stack) whereList() []string {
	var out []string
	if st.Bags > 0 {
		out = append(out, "bags")
	}
	if st.Bank > 0 {
		out = append(out, "bank")
	}
	return out
}

// tradable: a bound (Soulbound) or quest item cannot be sent.
func tradable(item map[string]any) bool { return itemkind.Tradable(item) }

// stacks gathers sendable items from bags and bank; the key is the lowercase name.
func stacks(s snapshot.Snapshot) map[string]*stack {
	out := map[string]*stack{}
	for key, line := range items.BagBankStock(s, tradable) {
		out[key] = &stack{*line}
	}
	return out
}

// reagentUses is material (lowercase) -> the recipes that use it, split by profession.
func reagentUses(s snapshot.Snapshot) map[string][]string {
	out := map[string][]string{}
	all := s.Map("recipes")
	for _, prof := range slices.Sorted(maps.Keys(all)) {
		for _, raw := range snapshot.AsList(snapshot.AsMap(all[prof])["recipes"]) {
			r := snapshot.AsMap(raw)
			rn, _ := r["name"].(string)
			for _, rr := range snapshot.AsList(r["reagents"]) {
				name := snapshot.ReagentName(snapshot.AsMap(rr))
				key := strings.ToLower(name)
				label := fmt.Sprintf("%s (%s)", rn, prof)
				if name != "" && !slices.Contains(out[key], label) {
					out[key] = append(out[key], label)
				}
			}
		}
	}
	return out
}

// weakProfessions is the character's professions (from the professions list) for which no recipes are saved.
func weakProfessions(s snapshot.Snapshot) []weakProfession {
	recipes := s.Map("recipes")
	var out []weakProfession
	for _, raw := range s.List("professions") {
		p := snapshot.AsMap(raw)
		name, _ := p["name"].(string)
		skill, ok := professions.SkillOf(p)
		if _, known := professionKinds[skill]; !ok || !known {
			continue
		}
		if _, has := recipes[name]; has {
			continue
		}
		out = append(out, weakProfession{name, skill})
	}
	return out
}

func weakNames(weak []weakProfession) []string {
	out := make([]string, len(weak))
	for i, w := range weak {
		out[i] = w.name
	}
	return out
}

// sortedByName returns the stocks in name order (case-insensitive).
func sortedByName(m map[string]*stack) []*stack {
	out := make([]*stack, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		out = append(out, m[k])
	}
	return out
}

// need describes why the recipient needs the material.
type need struct {
	evidence string
	reasons  []string
}

// needFor rates a material against the recipient; false when they do not need it.
func needFor(st *stack, uses map[string][]string, weak []weakProfession) (need, bool) {
	if recipes := uses[strings.ToLower(st.Name)]; len(recipes) > 0 {
		return need{EvidenceRecipe, recipes}, true
	}
	kind := itemkind.TradeGoodsKind(st.Item)
	if kind == "" {
		return need{}, false
	}
	label := kind
	if sub := st.SubType; sub != "" && sub != kind {
		label = sub // the client's own subtype name (e.g. Stoff on a German client)
	}
	var reasons []string
	for _, prof := range weak {
		if slices.Contains(professionKinds[prof.skill], kind) {
			reasons = append(reasons, fmt.Sprintf("%s (no saved recipes, matched by kind %q)", prof.name, label))
		}
	}
	if len(reasons) == 0 {
		return need{}, false
	}
	return need{EvidenceProfession, reasons}, true
}

// Plan compiles what to send to whom. Result: transfers (sender -> recipient with a list of materials).
func Plan(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	if db == nil || len(db.Characters) < 2 {
		return map[string]any{
			"transfers": []any{}, "count": 0,
			"note": "Mail suggestions need at least two characters in the dump. " + snapshot.AltsHint,
		}, nil
	}
	from, to := q.From, q.To
	for _, c := range []*string{&from, &to} {
		if *c == "" {
			continue
		}
		name, _, err := db.Lookup(*c)
		if err != nil {
			return nil, err
		}
		*c = name
	}
	query := strings.ToLower(q.Query)
	names := db.CharacterNames()

	transfers := []any{}
	anyRecipes, anyWeak := false, false
	var missingRecipes []string
	for _, sender := range names {
		if from != "" && sender != from {
			continue
		}
		mats := stacks(db.Characters[sender])
		for _, recipient := range names {
			if recipient == sender || (to != "" && recipient != to) {
				continue
			}
			rs := db.Characters[recipient]
			uses, weak := reagentUses(rs), weakProfessions(rs)
			if len(uses) > 0 {
				anyRecipes = true
			}
			if len(weak) > 0 {
				anyWeak = true
				if label := fmt.Sprintf("%s: %v", recipient, weakNames(weak)); !slices.Contains(missingRecipes, label) {
					missingRecipes = append(missingRecipes, label)
				}
			}
			held := stacks(rs)
			list := []any{}
			for _, st := range sortedByName(mats) {
				name := st.Name
				if query != "" && !strings.Contains(strings.ToLower(name), query) {
					continue
				}
				n, ok := needFor(st, uses, weak)
				if !ok {
					continue
				}
				e := map[string]any{"name": name, "count": st.Total(), "where": st.whereList(), "evidence": n.evidence, "reason": n.reasons}
				if h := held[strings.ToLower(name)]; h != nil {
					e["recipientHas"] = h.Total()
				}
				if n.evidence == EvidenceProfession {
					e["weakEvidence"] = true
					e["assumption"] = weakAssumption
				}
				list = append(list, e)
			}
			if len(list) > 0 {
				transfers = append(transfers, map[string]any{"from": sender, "to": recipient, "items": list, "itemKinds": len(list)})
			}
		}
	}
	sort.SliceStable(transfers, func(i, j int) bool {
		return len(transfers[i].(map[string]any)["items"].([]any)) > len(transfers[j].(map[string]any)["items"].([]any))
	})
	out := map[string]any{
		"transfers": transfers, "count": len(transfers),
		"assumptions": "Assumes both characters are on the same realm and the same faction (mail does not work between factions or realms). The server only reads data and sends nothing: the player sends the mail in game. Bound (Soulbound) and quest items are skipped. The bank is the state from the last visit, and the bags from the character's last dump.",
	}
	var hints []string
	if !anyRecipes {
		hints = append(hints, "No character has saved recipes. Open the profession window of each character in game (e.g. Leatherworking), then /mcp reload.")
	}
	if anyWeak {
		hints = append(hints, "For some professions there are no recipes in the dump, so items with evidence=profession are weaker evidence (only the kind of material). Open that profession window in game and type /mcp reload: "+strings.Join(missingRecipes, "; "))
	}
	if len(hints) > 0 {
		out["hint"] = hints
	}
	if len(transfers) == 0 {
		out["note"] = "No materials worth sending were found."
	}
	ages := map[string]any{}
	for _, n := range names {
		if m := fresh.SnapshotMeta(n, db.Characters[n]); m["ageMinutes"] != nil {
			ages[n] = m["ageMinutes"]
		}
	}
	if len(ages) > 0 {
		out["ageMinutes"] = ages
	}
	return out, nil
}
