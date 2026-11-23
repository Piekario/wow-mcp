// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package cooldowns

import (
	"maps"
	"slices"

	"wowmcp/internal/snapshot"
)

func read(s snapshot.Snapshot) []Source {
	var out []Source
	for _, raw := range s.List("cooldowns") {
		cd := snapshot.AsMap(raw)
		if at, ok := snapshot.AsInt(cd["readyAt"]); ok {
			name, _ := cd["name"].(string)
			kind, _ := cd["kind"].(string)
			out = append(out, Source{Name: name, Kind: kind, ReadyAt: at})
		}
	}
	recipes := s.Map("recipes")
	for _, prof := range slices.Sorted(maps.Keys(recipes)) {
		for _, raw := range snapshot.AsList(snapshot.AsMap(recipes[prof])["recipes"]) {
			r := snapshot.AsMap(raw)
			if at, ok := snapshot.AsInt(r["readyAt"]); ok {
				name, _ := r["name"].(string)
				out = append(out, Source{Name: name, Kind: "profession", Profession: prof, ReadyAt: at})
			}
		}
	}
	return out
}
