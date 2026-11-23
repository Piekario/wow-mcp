// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questroute

import (
	"wowmcp/internal/game/flights"
	"wowmcp/internal/snapshot"
)

const assumption = "The server has no world map or distances between zones, so the order is NOT the shortest route: " +
	"it is the current zone, then zones by the number of quests ready to turn in and of all quests (alphabetical on a tie), " +
	"and at the end red quests (5+ levels above the character) and zones made up only of them. " +
	"Flight points are matched by the zone's map ID when the dump has it (addon 0.10.0), otherwise from the point name (the part after the last comma or its abbreviation, e.g. Redridge -> Redridge Mountains), and the flight window usually knows only one continent, so a missing match does not prove there is no flight point; " +
	"the zone name in a quest may be a subzone or a city. Treat the order as a proposal and adjust it to the geography you know."

const noFlightsHint = "The dump has no flightPaths section: open the flight master window and run /mcp reload to see flight points next to zones."

var phaseLabels = map[Phase]string{
	CurrentZone: "current zone",
	ToDo:        "to do",
	Last:        "last (quests too hard)",
}

var statusLabels = map[Status]string{
	InProgress:    "in progress",
	ReadyToTurnIn: "ready to turn in",
	Failed:        "failed",
}

// View returns the proposed zone order. fresh is accepted for consistency with the neighbouring packages.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	_ = fresh
	return Analyse(s).Render(s)
}

func (r Route) Render(s snapshot.Snapshot) map[string]any {
	out := map[string]any{"assumption": assumption, "playerLevel": r.PlayerLevel}
	if r.CurrentZone != "" {
		out["currentZone"] = r.CurrentZone
	}
	if !r.HasLog {
		out["note"] = "The dump has no quest log. Run /mcp reload in game."
		out["route"] = []any{}
		return out
	}
	route := make([]any, 0, len(r.Zones))
	for i, z := range r.Zones {
		route = append(route, z.render(i+1))
	}
	out["route"] = route
	out["count"] = r.Total
	switch {
	case len(r.Zones) == 0:
		out["note"] = "The quest log is empty or the quests have no zone."
	case r.CurrentZone != "" && !r.Zones[0].Current:
		out["currentZoneNote"] = "The character is in a zone with no quests from the log (or the quests use a different zone name), so the route starts with the best zone by the numbers."
	}
	if !r.HasFlights {
		out["flightHint"] = noFlightsHint
	}
	flights.AddNameMatchNote(s, out, r.Nodes)
	return out
}

func (z *Zone) render(step int) map[string]any {
	list := make([]any, len(z.Quests))
	for i, q := range z.Quests {
		e := map[string]any{"title": q.Title, "level": q.Level, "difficulty": q.Difficulty, "status": statusLabels[q.Status]}
		if q.DoLast {
			e["doLast"] = true
		}
		list[i] = e
	}
	v := map[string]any{
		"step": step, "zone": z.Name, "phase": phaseLabels[z.Phase()], "current": z.Current,
		"readyToTurnIn": z.Ready, "inProgress": z.Doable, "tooHard": z.TooHard, "total": z.Total,
		"quests": list,
	}
	if len(z.Flights) > 0 {
		points := make([]any, len(z.Flights))
		for i, m := range z.Flights {
			points[i] = m.View()
		}
		v["flightPoints"] = points
	}
	return v
}
