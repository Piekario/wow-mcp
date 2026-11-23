// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package flights

import "wowmcp/internal/snapshot"

const missingHint = "The dump has no flightPaths section. Go to a flight master, open the window (flight map), then run /mcp reload so the server can see the flight points."

const assumption = "Flight points and quests are matched by the zone's map ID when both have one (addon 0.10.0; the same in every game language). Otherwise, and for points without a zone field, I match the zone from the point name (the part after the last comma, or, if there is none, " +
	"the whole name containing the zone name, or an abbreviation: Lakeshire, Redridge -> Redridge Mountains, Stormwind, Elwynn -> Stormwind City). The zone field of a quest is sometimes a subzone or a city, so a missing match does not prove there is no flight point nearby. " +
	"The list from the flight master window is the points known to the character; the meaning of the unreachable state is NOT confirmed: it is an assumption (probably no connection from the current point rather than an undiscovered one), to be verified in game."

// withoutNote explains why a zone may show no point despite a flight point being there.
const withoutNote = "The flight master window usually shows only the points of the continent where it was opened, so a zone from another continent " +
	"(e.g. Darkshore with the window opened in the Eastern Kingdoms) lands here even with a known flight point. A missing match does not prove there is no flight point."

// View matches flight points against quest zones.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Analyse(s, fresh).Render(s)
}

func (r Report) Render(s snapshot.Snapshot) map[string]any {
	out := map[string]any{"assumption": assumption}
	if r.Missing {
		out["note"] = missingHint
		out["hint"] = missingHint
		return out
	}
	if r.HasCaptured {
		out["capturedAt"] = snapshot.FormatTime(r.CapturedAt)
		if r.HasAge {
			out["capturedAge"] = snapshot.FormatAge(r.Age)
		}
	}
	out["knownFlightPoints"] = r.Known
	out["questZonesWithFlightPoint"] = renderZones(r.With, true)
	out["questZonesWithoutFlightPoint"] = renderZones(r.Without, false)
	if len(r.Without) > 0 {
		out["withoutNote"] = withoutNote
	}
	if r.NoQuestZones {
		out["note"] = "The quest log is empty or the quests have no zone, so there is nothing to match flight points against."
	}
	AddNameMatchNote(s, out, r.Nodes)
	return out
}

func renderZones(zones []ZoneMatches, withPoints bool) []any {
	out := []any{}
	for _, z := range zones {
		entry := map[string]any{"zone": z.Zone, "quests": len(z.Titles), "titles": z.Titles}
		if withPoints {
			points := make([]any, len(z.Matches))
			for i, m := range z.Matches {
				points[i] = m.View()
			}
			entry["flightPoints"] = points
		}
		out = append(out, entry)
	}
	return out
}

// AddNameMatchNote adds the localeNote when some flight points are matched to zones by their names.
func AddNameMatchNote(s snapshot.Snapshot, out map[string]any, nodes []Node) {
	if NeedsNameMatchNote(nodes) {
		s.AddEnglishOnlyNote(out, "Flight points without a zone ID or zone field are matched to zones by their names, with a table of English zone abbreviations")
	}
}

// View is the match as in responses: name, state, matchedBy, and zoneFieldUnmatched when the point's own zone field
// or map ID matched no quest zone (so the name decided).
func (m Match) View() map[string]any {
	out := map[string]any{"name": m.Node.Name, "state": m.Node.State, "matchedBy": m.By}
	if m.Node.ZoneUnmatched {
		out["zoneFieldUnmatched"] = true
	}
	return out
}
