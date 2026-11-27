// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package lookup

import (
	"wowmcp/internal/snapshot"
)

// readEntries reads the lookups section, newest first as the addon wrote it.
func readEntries(s snapshot.Snapshot) []Entry {
	var out []Entry
	for _, raw := range s.List("lookups") {
		if e, ok := readEntry(snapshot.AsMap(raw)); ok {
			out = append(out, e)
		}
	}
	return out
}

func readEntry(m map[string]any) (Entry, bool) {
	id, ok := snapshot.AsInt(m["id"])
	kind := Kind(str(m["kind"]))
	if !ok || (kind != Item && kind != Spell) {
		return Entry{}, false
	}
	e := Entry{Kind: kind, ID: id, Name: str(m["name"]), Query: str(m["query"]), Rank: str(m["rank"])}
	e.At, _ = snapshot.AsInt(m["at"])
	e.Description, e.DescriptionSource = str(m["description"]), str(m["descriptionSource"])
	e.DescriptionMissing = m["descriptionMissing"] == true
	e.CastTimeMs, e.HasCastTime = snapshot.AsInt(m["castTimeMs"])
	e.MinRange, _ = snapshot.AsInt(m["minRange"])
	e.MaxRange, _ = snapshot.AsInt(m["maxRange"])
	for _, raw := range snapshot.AsList(m["costs"]) {
		c := snapshot.AsMap(raw)
		amount, _ := snapshot.AsInt(c["amount"])
		e.Costs = append(e.Costs, Cost{Power: str(c["power"]), Amount: amount})
	}
	e.Quality, e.HasQuality = snapshot.AsInt(m["quality"])
	e.ItemLevel, _ = snapshot.AsInt(m["itemLevel"])
	e.RequiredLevel, _ = snapshot.AsInt(m["requiredLevel"])
	e.Type, e.SubType, e.EquipLoc = str(m["type"]), str(m["subType"]), str(m["equipLoc"])
	e.SellPrice, _ = snapshot.AsInt(m["sellPrice"])
	e.Stats = readStats(m["stats"])
	for _, line := range snapshot.AsList(m["tooltip"]) {
		e.Tooltip = append(e.Tooltip, str(line))
	}
	return e, true
}

func readStats(raw any) map[string]float64 {
	stats := map[string]float64{}
	for name, v := range snapshot.AsMap(raw) {
		if n, ok := snapshot.AsFloat(v); ok {
			stats[name] = n
		}
	}
	return stats
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
