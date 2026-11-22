// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package reputation covers faction reputation: standing and progress to the next one.
package reputation

import (
	"fmt"
	"math"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query holds the wow_reputation filters.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Query     string `json:"query,omitempty" jsonschema:"faction name fragment, e.g. Darnassus"`
	Standing  string `json:"standing,omitempty" jsonschema:"only factions with this standing, e.g. Friendly, Honored, Neutral"`
}

// View describes factions with progress in the current standing and how much is missing for the next one.
func View(s snapshot.Snapshot, q Query) map[string]any {
	factions := []any{}
	for _, raw := range s.List("reputation") {
		f := snapshot.AsMap(raw)
		name, _ := f["name"].(string)
		standing := fmt.Sprint(f["standing"])
		if q.Query != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(q.Query)) {
			continue
		}
		if q.Standing != "" && !strings.EqualFold(standing, q.Standing) {
			continue
		}
		factions = append(factions, faction(name, f))
	}
	out := map[string]any{"factions": factions, "count": len(factions)}
	if _, ok := s["reputation"]; !ok {
		out["note"] = "The dump has no reputation section. Run /mcp reload in game."
	}
	return out
}

// faction is one faction with progress between the floor and ceiling thresholds of the current standing.
func faction(name string, f map[string]any) map[string]any {
	entry := map[string]any{"name": name, "standing": f["standing"]}
	value, ok1 := snapshot.AsInt(f["value"])
	floor, ok2 := snapshot.AsInt(f["floor"])
	ceiling, ok3 := snapshot.AsInt(f["ceiling"])
	if ok1 && ok2 && ok3 && ceiling > floor {
		entry["progress"] = fmt.Sprintf("%d/%d", value-floor, ceiling-floor)
		entry["percent"] = math.Round(float64(value-floor)*1000/float64(ceiling-floor)) / 10
		entry["toNextStanding"] = ceiling - value
	} else if ok1 {
		entry["value"] = value
	}
	return entry
}
