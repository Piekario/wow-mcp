// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package questroute proposes an order for visiting zones with quests from the log (the quests section):
// first the character's current zone (character.zone), then zones by the number of quests ready to turn in
// and of all quests, and at the end quests that are too hard (red) and zones made up only of them.
// Each zone gets the known flight points attached (the flightPaths section).
//
// The server knows neither the world map nor the distances between zones, so the order is NOT the
// shortest route: it follows only from the quest counts and the current zone. The assumption field says so.
package questroute

import (
	"sort"
	"strings"

	"wowmcp/internal/game/flights"
	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

const assumption = "The server has no world map or distances between zones, so the order is NOT the shortest route: " +
	"it is the current zone, then zones by the number of quests ready to turn in and of all quests (alphabetical on a tie), " +
	"and at the end red quests (5+ levels above the character) and zones made up only of them. " +
	"Flight points are matched by the zone's map ID when the dump has it (addon 0.10.0), otherwise from the point name (the part after the last comma or its abbreviation, e.g. Redridge -> Redridge Mountains), and the flight window usually knows only one continent, so a missing match does not prove there is no flight point; " +
	"the zone name in a quest may be a subzone or a city. Treat the order as a proposal and adjust it to the geography you know."

const noFlightsHint = "The dump has no flightPaths section: open the flight master window and run /mcp reload to see flight points next to zones."

const (
	phaseCurrent = "current zone"
	phaseMain    = "to do"
	phaseLater   = "last (quests too hard)"
)

type zone struct {
	name                          string
	mapID                         int64 // UiMapID of the zone from the quests (addon 0.10.0), 0 = unknown
	quests                        []map[string]any
	ready, doable, tooHard, total int
	current                       bool
	flights                       []any
}

// View returns the proposed zone order. fresh is accepted for consistency with the neighbouring packages.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	_ = fresh
	out := map[string]any{"assumption": assumption}
	level, _ := s.LevelAndMoney()
	green := greenRange(s, level)
	c := s.Character()
	current, _ := c["zone"].(string)
	if current != "" {
		out["currentZone"] = current
	}
	out["playerLevel"] = level

	raw, has := s["quests"]
	if !has {
		out["note"] = "The dump has no quest log. Run /mcp reload in game."
		out["route"] = []any{}
		return out
	}
	zones := groupZones(snapshot.AsList(raw), level, green, current, currentMapID(c))
	nodes, haveFlights := flightNodes(s)
	questZones := flights.QuestZones{}
	for _, z := range zones {
		questZones[z.name] = z.mapID
	}
	nodes = questZones.Guessable(nodes)
	for _, z := range zones {
		z.flights = matchFlights(z.name, z.mapID, nodes)
	}
	order(zones)

	route := make([]any, 0, len(zones))
	total := 0
	for i, z := range zones {
		total += z.total
		route = append(route, z.view(i+1))
	}
	out["route"] = route
	out["count"] = total
	switch {
	case len(zones) == 0:
		out["note"] = "The quest log is empty or the quests have no zone."
	case current != "" && !zones[0].current:
		out["currentZoneNote"] = "The character is in a zone with no quests from the log (or the quests use a different zone name), so the route starts with the best zone by the numbers."
	}
	if !haveFlights {
		out["flightHint"] = noFlightsHint
	}
	flights.AddNameMatchNote(s, out, nodes)
	return out
}

// currentMapID is the map the character stands on (character.mapID), 0 when the dump has none.
func currentMapID(c map[string]any) int64 {
	id, _ := snapshot.AsInt(c["mapID"])
	return id
}

func greenRange(s snapshot.Snapshot, level int64) int64 {
	if g, ok := snapshot.AsInt(s.Character()["questGreenRange"]); ok && g > 0 {
		return g
	}
	return quests.GreenRange(level)
}

// groupZones groups quests by zone; quests without a zone go to "(no zone)".
func groupZones(list []any, level, green int64, current string, currentID int64) []*zone {
	byName := map[string]*zone{}
	var zones []*zone
	for _, raw := range list {
		q := snapshot.AsMap(raw)
		name, _ := q["zone"].(string)
		if name == "" {
			name = "(no zone)"
		}
		z := byName[name]
		if z == nil {
			z = &zone{name: name, current: current != "" && strings.EqualFold(name, current)}
			byName[name] = z
			zones = append(zones, z)
		}
		if id, ok := snapshot.AsInt(q["zoneMapID"]); ok && id > 0 && z.mapID == 0 {
			z.mapID = id
			// The map ID is the same in every client language, so it identifies the current zone without names.
			z.current = z.current || (currentID > 0 && id == currentID)
		}
		z.add(q, level, green)
	}
	return zones
}

func (z *zone) add(q map[string]any, level, green int64) {
	ql, _ := snapshot.AsInt(q["level"])
	diff := quests.Difficulty(ql, level, green)
	title, _ := q["title"].(string)
	e := map[string]any{"title": title, "level": ql, "difficulty": diff}
	z.total++
	switch {
	case q["failed"] == true:
		e["status"] = "failed"
	case q["complete"] == true:
		e["status"] = "ready to turn in"
		z.ready++
	case diff == "red":
		e["status"] = "in progress"
		e["doLast"] = true
		z.tooHard++
	default:
		e["status"] = "in progress"
		z.doable++
	}
	z.quests = append(z.quests, e)
}

// onlyTooHard: a zone with nothing to turn in or do, but with red quests.
func (z *zone) onlyTooHard() bool { return z.ready == 0 && z.doable == 0 && z.tooHard > 0 }

// order sorts: current zone, zones with work, zones with only red quests.
func order(zones []*zone) {
	rank := func(z *zone) int {
		switch {
		case z.current:
			return 0
		case z.onlyTooHard():
			return 2
		}
		return 1
	}
	sort.SliceStable(zones, func(i, j int) bool {
		a, b := zones[i], zones[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		if a.ready != b.ready {
			return a.ready > b.ready
		}
		if a.total != b.total {
			return a.total > b.total
		}
		return a.name < b.name
	})
}

func (z *zone) view(step int) map[string]any {
	phase := phaseMain
	switch {
	case z.current:
		phase = phaseCurrent
	case z.onlyTooHard():
		phase = phaseLater
	}
	// Quests in a zone: ready, in progress, red ones last; within groups from the lowest level.
	sort.SliceStable(z.quests, func(i, j int) bool {
		a, b := z.quests[i], z.quests[j]
		if ra, rb := questRank(a), questRank(b); ra != rb {
			return ra < rb
		}
		return a["level"].(int64) < b["level"].(int64)
	})
	list := make([]any, len(z.quests))
	for i, q := range z.quests {
		list[i] = q
	}
	v := map[string]any{
		"step": step, "zone": z.name, "phase": phase, "current": z.current,
		"readyToTurnIn": z.ready, "inProgress": z.doable, "tooHard": z.tooHard, "total": z.total,
		"quests": list,
	}
	if len(z.flights) > 0 {
		v["flightPoints"] = z.flights
	}
	return v
}

func questRank(q map[string]any) int {
	switch {
	case q["status"] == "ready to turn in":
		return 0
	case q["doLast"] == true:
		return 2
	}
	return 1
}

// flightNodes reads the known flight points; the second result says whether the dump has a flightPaths section.
func flightNodes(s snapshot.Snapshot) ([]flights.Node, bool) {
	raw, has := s.Map("flightPaths")["nodes"]
	if !has {
		return nil, false
	}
	return flights.ParseNodes(snapshot.AsList(raw)), true
}

// matchFlights matches flight points to a zone using the shared game/flights logic.
func matchFlights(zoneName string, zoneMapID int64, nodes []flights.Node) []any {
	var out []any
	for _, m := range flights.MatchZoneID(zoneName, zoneMapID, nodes) {
		out = append(out, m.View())
	}
	return out
}
