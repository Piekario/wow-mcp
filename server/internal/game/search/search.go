// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package search finds text across a whole dump of one character: quests, items
// in bags, bank and worn, skills, trainer and vendor offers, recipes,
// mail, reputation, auctions and a few others. Instead of a separate loop for each section there is
// one generic walk over the section structure and a table saying which fields to search.
package search

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_search.
type Query struct {
	Query     string `json:"query" jsonschema:"text to search for (case-insensitive), e.g. an item, quest, skill or zone name or a fragment of a description"`
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
}

// maxHitsPerGroup is how many hits from one section we show; the rest are only counted.
const maxHitsPerGroup = 20

// spec describes how to search one dump section.
type spec struct {
	section  string   // section key in the dump
	fields   []string // text fields (a string or a list of strings) we search in
	describe []string // fields that make up the short summary of a hit
	// lastVisit is the section with the last visit (e.g. vendor), whose copy also lies in this section's map (vendors);
	// the copy is skipped so the same hit does not appear twice.
	lastVisit string
}

// labelKeys: which field names the record (the first found); also an element of the hit path.
var labelKeys = []string{"title", "name", "subject", "text", "npc"}

// moneyKeys are fields with an amount in copper, shown as in the game.
var moneyKeys = map[string]bool{
	"minBuyout": true, "minBid": true, "buyout": true, "bid": true, "sellPrice": true,
	"priceCopper": true, "costCopper": true, "moneyCopper": true, "codCopper": true,
}

// specs are the searched sections in response order.
var specs = []spec{
	{"quests", []string{"title", "text", "zone", "name"}, []string{"level", "zone", "complete"}, ""}, // name: quest rewards (quests[].rewards)
	{"questOffer", []string{"title", "text", "name", "tooltip"}, []string{"npc", "xp"}, ""},
	{"questTurnIn", []string{"title", "text", "name", "tooltip"}, []string{"npc", "xp"}, ""},
	{"bags", []string{"name", "type", "subType", "tooltip"}, []string{"count", "type", "subType"}, ""},
	{"bank", []string{"name", "type", "subType", "tooltip"}, []string{"count", "type", "subType"}, ""},
	{"equipment", []string{"name", "slot", "type", "subType", "tooltip"}, []string{"slot", "itemLevel", "type"}, ""},
	{"spells", []string{"name", "rank", "tab"}, []string{"rank", "tab"}, ""},
	{"trainer", []string{"name", "rank"}, []string{"rank", "category", "costCopper", "levelRequired"}, ""},
	{"vendor", []string{"name", "type", "subType", "tooltip"}, []string{"priceCopper", "type", "requiredLevel"}, ""},
	{"vendors", []string{"name", "type", "subType", "tooltip"}, []string{"priceCopper", "type", "requiredLevel"}, "vendor"},
	{"trainers", []string{"name", "rank"}, []string{"rank", "category", "costCopper", "levelRequired"}, "trainer"},
	{"loot", []string{"name"}, []string{"count"}, ""},
	{"stable", []string{"name", "family", "specialization"}, []string{"family", "level", "slot"}, ""},
	{"recipes", []string{"name", "group"}, []string{"difficulty", "levelRequired", "canCraft", "have", "need", "trainingPoints"}, ""},
	{"mail", []string{"subject", "sender", "name"}, []string{"sender", "moneyCopper", "count"}, ""},
	{"reputation", []string{"name", "standing"}, []string{"standing", "value"}, ""},
	{"auctions", []string{"name"}, []string{"minBuyout", "minBid", "buyout", "bid", "quantity", "count", "timeLeftSeconds"}, ""},
	{"professions", []string{"name"}, []string{"rank", "maxRank"}, ""},
	{"currencies", []string{"name", "group"}, []string{"quantity", "group"}, ""},
	{"macros", []string{"name", "body"}, []string{"scope"}, ""},
	{"flightPaths", []string{"name"}, []string{"state"}, ""},
}

type hit struct {
	label, field, text, summary string
	path                        []string
}

func (h hit) view() map[string]any {
	out := map[string]any{"label": h.label, "path": h.path, "matchedIn": h.field}
	if h.text != "" && h.text != h.label {
		out["matchedText"] = h.text
	}
	if h.summary != "" {
		out["summary"] = h.summary
	}
	return out
}

// Run searches the dump. Empty text is an error because it would return the whole dump.
func Run(s snapshot.Snapshot, q Query) (map[string]any, error) {
	needle := strings.ToLower(strings.TrimSpace(q.Query))
	if needle == "" {
		return nil, fmt.Errorf("provide the text to search for in the query parameter")
	}
	groups := []any{}
	var total int
	var searched, missing []string
	for _, sp := range specs {
		value, ok := s[sp.section]
		if !ok {
			missing = append(missing, sp.section)
			continue
		}
		searched = append(searched, sp.section)
		hits := sp.search(sp.withoutLastVisitCopy(s, value), needle)
		if len(hits) == 0 {
			continue
		}
		total += len(hits)
		groups = append(groups, groupView(sp.section, hits))
	}
	out := map[string]any{
		"query":     q.Query,
		"totalHits": total,
		"groups":    groups,
		"truncated": anyTruncated(groups),
		"searched":  searched,
		"note":      "Bank, vendors, trainers, stable, quest windows, recipes, mail and auctions are the state from the last visits to those windows (check their age in the relevant tools); loot is the loot since the previous save.",
	}
	if len(missing) > 0 {
		out["notInSnapshot"] = missing
	}
	if total == 0 {
		out["note"] = "Nothing found in the searched sections. " + out["note"].(string)
	}
	return out, nil
}

func groupView(section string, hits []hit) map[string]any {
	shown := hits
	if len(shown) > maxHitsPerGroup {
		shown = shown[:maxHitsPerGroup]
	}
	views := make([]any, len(shown))
	for i, h := range shown {
		views[i] = h.view()
	}
	return map[string]any{
		"section": section, "count": len(hits), "shown": len(shown),
		"truncated": len(hits) > len(shown), "hits": views,
	}
}

func anyTruncated(groups []any) bool {
	for _, g := range groups {
		if snapshot.AsMap(g)["truncated"] == true {
			return true
		}
	}
	return false
}

// withoutLastVisitCopy removes from the visits map (vendors, trainers) the entry that is a copy of the last visit (same NPC and capture time).
func (sp spec) withoutLastVisitCopy(s snapshot.Snapshot, value any) any {
	recent := s.Map(sp.lastVisit)
	visits, ok := value.(map[string]any)
	if sp.lastVisit == "" || recent == nil || !ok {
		return value
	}
	out := make(map[string]any, len(visits))
	for npc, raw := range visits {
		entry := snapshot.AsMap(raw)
		if entry["npc"] == recent["npc"] && entry["capturedAt"] == recent["capturedAt"] {
			continue
		}
		out[npc] = raw
	}
	return out
}

// search walks the section value and collects hits.
func (sp spec) search(value any, needle string) []hit {
	var hits []hit
	walk(value, []string{sp.section}, func(trail []string, m map[string]any) {
		if field, text, ok := sp.match(m, needle); ok {
			label := labelOf(m)
			if label == "" {
				label = text
			}
			hits = append(hits, hit{label: label, path: trail, field: field, text: text, summary: sp.summary(m)})
		}
	})
	return hits
}

// match checks the record's fields in order; it returns the first matching field and the matching text.
func (sp spec) match(m map[string]any, needle string) (field, text string, ok bool) {
	for _, f := range sp.fields {
		for _, candidate := range texts(m[f]) {
			if strings.Contains(strings.ToLower(candidate), needle) {
				return f, candidate, true
			}
		}
	}
	return "", "", false
}

// summary builds a short description of the hit from the describe fields: strings unchanged, numbers with the field name, amounts as in the game.
func (sp spec) summary(m map[string]any) string {
	var parts []string
	for _, key := range sp.describe {
		v, ok := m[key]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case string:
			parts = append(parts, x)
		case bool:
			if x {
				parts = append(parts, key)
			}
		default:
			if n, isNum := snapshot.AsInt(v); isNum {
				if moneyKeys[key] {
					parts = append(parts, key+" "+snapshot.Money(n).String())
				} else {
					parts = append(parts, fmt.Sprintf("%s %d", key, n))
				}
			}
		}
	}
	return strings.Join(parts, ", ")
}

// walk visits every table in the value (lists and nested tables, in a stable key order).
// trail is the labels of the ancestor records, together with the visited table if it has a label.
func walk(value any, trail []string, visit func(trail []string, m map[string]any)) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			walk(item, trail, visit)
		}
	case map[string]any, snapshot.Snapshot:
		m := snapshot.AsMap(v)
		here := trail
		if label := labelOf(m); label != "" {
			here = append(append([]string{}, trail...), label)
		}
		visit(here, m)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walk(m[k], here, visit)
		}
	}
}

// labelOf returns the record's name: the first non-empty text field from labelKeys.
func labelOf(m map[string]any) string {
	for _, k := range labelKeys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// texts returns the field value as a list of strings (a single string or a list of strings, e.g. tooltip).
func texts(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, item := range x {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}
