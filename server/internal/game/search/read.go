// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package search

import (
	"sort"

	"wowmcp/internal/snapshot"
)

// labelKeys: which field names the record (the first found); also an element of the hit path.
var labelKeys = []string{"title", "name", "subject", "text", "npc"}

// readRecords walks the section and turns every table in it into a Record. False when the dump has no such section.
func readRecords(s snapshot.Snapshot, sp spec) ([]Record, bool) {
	value, ok := s[sp.section]
	if !ok {
		return nil, false
	}
	value = sp.withoutLastVisitCopy(s, value)
	var records []Record
	walk(value, []string{sp.section}, func(trail []string, m map[string]any) {
		records = append(records, sp.record(trail, m))
	})
	return records, true
}

func (sp spec) record(trail []string, m map[string]any) Record {
	rec := Record{Trail: trail, Label: labelOf(m)}
	for _, f := range sp.fields {
		rec.Texts = append(rec.Texts, FieldTexts{Field: f, Values: texts(m[f])})
	}
	for _, key := range sp.describe {
		if fact, ok := factOf(key, m[key]); ok {
			rec.Facts = append(rec.Facts, fact)
		}
	}
	return rec
}

func factOf(key string, v any) (Fact, bool) {
	switch x := v.(type) {
	case nil:
		return Fact{}, false
	case string:
		return Fact{Key: key, Kind: FactText, Text: x}, true
	case bool:
		return Fact{Key: key, Kind: FactFlag, Flag: x}, true
	}
	n, ok := snapshot.AsInt(v)
	return Fact{Key: key, Kind: FactNumber, N: n}, ok
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
