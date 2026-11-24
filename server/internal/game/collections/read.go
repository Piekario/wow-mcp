// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package collections

import "wowmcp/internal/snapshot"

func asInt(v any) int64 {
	n, _ := snapshot.AsInt(v)
	return n
}

func asText(v any) string {
	s, _ := v.(string)
	return s
}

func read(s snapshot.Snapshot) Section {
	var section Section
	if m := s.Map("titles"); m != nil {
		section.HasTitles = true
		section.Titles = Titles{Total: asInt(m["total"]), CurrentID: asInt(m["currentID"]), Current: asText(m["current"])}
		for _, raw := range snapshot.AsList(m["known"]) {
			t := snapshot.AsMap(raw)
			section.Titles.Known = append(section.Titles.Known, Title{ID: asInt(t["id"]), Name: asText(t["name"])})
		}
	}
	if m := s.Map("collections"); m != nil {
		section.HasCollections = true
		section.Mounts = readGroup(snapshot.AsMap(m["mounts"]))
		section.Companions = readGroup(snapshot.AsMap(m["companions"]))
	}
	return section
}

func readGroup(m map[string]any) Group {
	if m == nil {
		return Group{}
	}
	g := Group{Present: true, Source: asText(m["source"]), NotListed: asInt(m["notListed"])}
	for _, raw := range snapshot.AsList(m["list"]) {
		c := snapshot.AsMap(raw)
		g.List = append(g.List, Collectible{
			Name: asText(c["name"]), Species: asText(c["species"]),
			ID: asInt(c["id"]), SpellID: asInt(c["spellID"]), Level: asInt(c["level"]),
		})
	}
	return g
}
