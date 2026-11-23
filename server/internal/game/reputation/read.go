// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reputation

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

func read(s snapshot.Snapshot) (factions []Faction, saved bool) {
	_, saved = s["reputation"]
	for _, raw := range s.List("reputation") {
		f := snapshot.AsMap(raw)
		name, _ := f["name"].(string)
		faction := Faction{Name: name, Standing: f["standing"], StandingText: fmt.Sprint(f["standing"])}
		faction.Value, faction.HasValue = snapshot.AsInt(f["value"])
		faction.Floor, faction.HasFloor = snapshot.AsInt(f["floor"])
		faction.Ceiling, faction.HasCeiling = snapshot.AsInt(f["ceiling"])
		factions = append(factions, faction)
	}
	return factions, saved
}
