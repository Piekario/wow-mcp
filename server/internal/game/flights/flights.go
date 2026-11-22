// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package flights matches known flight points (the flightPaths section, saved when the flight
// master window is opened) against the zones where the character has quests (the quests section): which zone has a
// known flight point and which has none. Since addon 0.10.0 flight points and quests carry the zone's map ID
// (zoneMapID), which is the same in every client language, and matching goes by it first. For older dumps and points
// without an ID the match falls back to the zone name: the zone field of the point, or the zone extracted from its name
// ("Auberdine, Darkshore" -> Darkshore); the response says so in assumption. A zone field that is a continent name
// (addon 0.7.0 wrote "Eastern Kingdoms" for every point) counts as no zone, and a point whose zone field or map ID
// matches no quest zone at all is also matched by its name (QuestZones.Guessable).
package flights

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

const missingHint = "The dump has no flightPaths section. Go to a flight master, open the window (flight map), then run /mcp reload so the server can see the flight points."

const assumption = "Flight points and quests are matched by the zone's map ID when both have one (addon 0.10.0; the same in every game language). Otherwise, and for points without a zone field, I match the zone from the point name (the part after the last comma, or, if there is none, " +
	"the whole name containing the zone name, or an abbreviation: Lakeshire, Redridge -> Redridge Mountains, Stormwind, Elwynn -> Stormwind City). The zone field of a quest is sometimes a subzone or a city, so a missing match does not prove there is no flight point nearby. " +
	"The list from the flight master window is the points known to the character; the meaning of the unreachable state is NOT confirmed: it is an assumption (probably no connection from the current point rather than an undiscovered one), to be verified in game."

// withoutNote explains why a zone may show no point despite a flight point being there.
const withoutNote = "The flight master window usually shows only the points of the continent where it was opened, so a zone from another continent " +
	"(e.g. Darkshore with the window opened in the Eastern Kingdoms) lands here even with a known flight point. A missing match does not prove there is no flight point."

// Matching values say how a flight point was matched to a zone.
const (
	byZoneMapID   = "zone map ID"
	byZoneField   = "zone field"
	byNameSuffix  = "name suffix"
	byNameContain = "name contains zone"
	byShortName   = "abbreviated zone name"
)

// Node is a known flight point from the flightPaths section.
type Node struct {
	Name, State, Zone string
	Continent         string // the continent when that is all the dump says (the continent field, or a continent name in zone)
	NodeID            int64  // the client's flight point ID (0 = unknown)
	ZoneMapID         int64  // UiMapID of the point's zone (0 = unknown; addon before 0.10.0, or the client did not say)
	// ZoneUnmatched: the point's zone field or map ID matched no quest zone, so it is matched by its name (Guessable).
	ZoneUnmatched bool
}

// continentNames are the continent names that addon 0.7.0 wrote as the zone of flight points (from the map at the
// point's position on the flight map). English names: older dumps all came from English clients; addon 0.12.0 writes
// a continent into its own field instead.
var continentNames = map[string]bool{
	"eastern kingdoms": true, "kalimdor": true, "outland": true, "northrend": true, "pandaria": true,
	"broken isles": true, "azeroth": true, "the maelstrom": true, "draenor": true, "zandalar": true,
	"kul tiras": true, "the shadowlands": true, "dragon isles": true, "khaz algar": true,
}

// isContinentName says whether a zone field actually holds a continent name.
func isContinentName(zone string) bool {
	return continentNames[strings.ToLower(strings.TrimSpace(zone))]
}

// QuestZones is the set of quest zones flight points are matched against: name -> map ID (0 when unknown).
type QuestZones map[string]int64

// Guessable returns the nodes with the zone field and map ID cleared (ZoneUnmatched set) for every node whose zone field
// or map ID matches none of the quest zones: such a zone is no help, and the node is matched by its name instead
// (the name-based guess used for older dumps). Nodes that match some quest zone are returned unchanged.
func (q QuestZones) Guessable(nodes []Node) []Node {
	names, ids := map[string]bool{}, map[int64]bool{}
	for zone, id := range q {
		names[strings.ToLower(zone)] = true
		if id > 0 {
			ids[id] = true
		}
	}
	out := make([]Node, len(nodes))
	for i, n := range nodes {
		out[i] = n
		if n.Zone == "" && n.ZoneMapID == 0 {
			continue
		}
		if (n.ZoneMapID > 0 && ids[n.ZoneMapID]) || (n.Zone != "" && names[strings.ToLower(n.Zone)]) {
			continue
		}
		out[i].Zone, out[i].ZoneMapID, out[i].ZoneUnmatched = "", 0, true
	}
	return out
}

// View matches flight points against quest zones.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"assumption": assumption}
	section := s.Map("flightPaths")
	rawNodes, has := section["nodes"]
	if !has {
		out["note"] = missingHint
		out["hint"] = missingHint
		return out
	}
	nodes := ParseNodes(snapshot.AsList(rawNodes))
	if at, ok := snapshot.AsInt(section["capturedAt"]); ok {
		out["capturedAt"] = snapshot.FormatTime(at)
		if fresh.Now != nil {
			out["capturedAge"] = snapshot.FormatAge(fresh.Age(at))
		}
	}
	out["knownFlightPoints"] = len(nodes)

	byZone, ids, order := questZones(s.List("quests"))
	nodes = QuestZones(ids).Guessable(nodes)
	var with, without []any
	for _, zone := range order {
		titles := byZone[zone]
		entry := map[string]any{"zone": zone, "quests": len(titles), "titles": titles}
		matches := MatchZoneID(zone, ids[zone], nodes)
		if len(matches) == 0 {
			without = append(without, entry)
			continue
		}
		points := make([]any, len(matches))
		for i, m := range matches {
			points[i] = m.View()
		}
		entry["flightPoints"] = points
		with = append(with, entry)
	}
	out["questZonesWithFlightPoint"] = nonNil(with)
	out["questZonesWithoutFlightPoint"] = nonNil(without)
	if len(without) > 0 {
		out["withoutNote"] = withoutNote
	}
	if len(order) == 0 {
		out["note"] = "The quest log is empty or the quests have no zone, so there is nothing to match flight points against."
	}
	AddNameMatchNote(s, out, nodes)
	return out
}

// AddNameMatchNote adds the localeNote when some flight points have neither a zone map ID nor a zone field (or only one
// that matched no quest zone): those are matched to zones by their names, with a table of English zone abbreviations.
func AddNameMatchNote(s snapshot.Snapshot, out map[string]any, nodes []Node) {
	for _, n := range nodes {
		if n.Zone == "" && n.ZoneMapID == 0 {
			s.AddEnglishOnlyNote(out, "Flight points without a zone ID or zone field are matched to zones by their names, with a table of English zone abbreviations")
			return
		}
	}
}

func nonNil(l []any) []any {
	if l == nil {
		return []any{}
	}
	return l
}

// isRetiredNode says whether the point is a retired game data entry (zzOLD prefix, e.g. "zzOLDRiverglades, Farholde Keep"
// next to the real "Farholde Keep, Riverglades" in a Forever 1.60.1 dump).
func isRetiredNode(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "zzold")
}

// ParseNodes reads the flightPaths.nodes list (skips entries without a name and retired zzOLD entries).
func ParseNodes(list []any) []Node {
	var out []Node
	for _, raw := range list {
		m := snapshot.AsMap(raw)
		name, _ := m["name"].(string)
		if name == "" || isRetiredNode(name) {
			continue
		}
		n := Node{Name: name}
		n.State, _ = m["state"].(string)
		n.Zone, _ = m["zone"].(string)
		n.Continent, _ = m["continent"].(string)
		if isContinentName(n.Zone) {
			n.Continent, n.Zone = n.Zone, ""
		}
		n.NodeID, _ = snapshot.AsInt(m["nodeID"])
		n.ZoneMapID, _ = snapshot.AsInt(m["zoneMapID"])
		out = append(out, n)
	}
	return out
}

// questZones groups quest titles by zone; ids has every zone with its map ID (the first quest that has one, else 0);
// order lists zones by most quests, then alphabetically.
func questZones(quests []any) (map[string][]string, map[string]int64, []string) {
	byZone := map[string][]string{}
	ids := map[string]int64{}
	for _, raw := range quests {
		q := snapshot.AsMap(raw)
		zone, _ := q["zone"].(string)
		if zone == "" {
			continue
		}
		title, _ := q["title"].(string)
		byZone[zone] = append(byZone[zone], title)
		if _, seen := ids[zone]; !seen {
			ids[zone] = 0
		}
		if id, ok := snapshot.AsInt(q["zoneMapID"]); ok && id > 0 && ids[zone] == 0 {
			ids[zone] = id
		}
	}
	order := make([]string, 0, len(byZone))
	for zone := range byZone {
		order = append(order, zone)
	}
	sort.Slice(order, func(i, j int) bool {
		if a, b := len(byZone[order[i]]), len(byZone[order[j]]); a != b {
			return a > b
		}
		return order[i] < order[j]
	})
	return byZone, ids, order
}

// Match is a flight point matched to a zone and the way it matched (By: zone field, name suffix, name contains zone).
type Match struct {
	Node Node
	By   string
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

// MatchZoneID matches flight points to a zone by its map ID first: a point that has a zone map ID is a match when the
// IDs are equal and never otherwise (no name guessing for it); points without an ID, and a zone without one
// (zoneMapID <= 0), go through MatchZone by name. Shared by wow_flights and wow_quest_route.
func MatchZoneID(zone string, zoneMapID int64, nodes []Node) []Match {
	if zoneMapID <= 0 {
		return MatchZone(zone, nodes)
	}
	var out []Match
	var byName []Node
	for _, n := range nodes {
		switch {
		case n.ZoneMapID == zoneMapID:
			out = append(out, Match{n, byZoneMapID})
		case n.ZoneMapID == 0:
			byName = append(byName, n)
		}
	}
	return append(out, MatchZone(zone, byName)...)
}

// MatchZone returns flight points matching a zone: by the zone field (addon 0.7.0), and for entries without it
// by the name suffix or by a comma-less name containing the zone. Shared by wow_flights and wow_quest_route.
func MatchZone(zone string, nodes []Node) []Match {
	z := strings.ToLower(zone)
	var out []Match
	for _, n := range nodes {
		name := strings.ToLower(n.Name)
		switch {
		case n.Zone != "" && strings.ToLower(n.Zone) == z:
			out = append(out, Match{n, byZoneField})
		case n.Zone == "" && suffixZone(name) == z:
			out = append(out, Match{n, byNameSuffix})
		case n.Zone == "" && !strings.Contains(name, ",") && strings.Contains(name, z):
			out = append(out, Match{n, byNameContain})
		case n.Zone == "" && shortNameOf(name, z):
			out = append(out, Match{n, byShortName})
		}
	}
	return out
}

// shortNameOf says whether the point is named with an abbreviation of the zone: the suffix is the start of the zone name
// ("lakeshire, redridge" -> "redridge mountains") or the part before the comma is a city
// ("stormwind, elwynn" -> "stormwind city", "ironforge, dun morogh" -> "ironforge").
func shortNameOf(name, zone string) bool {
	suffix := suffixZone(name)
	if suffix == "" {
		return false
	}
	head := strings.TrimSpace(name[:strings.LastIndex(name, ",")])
	return wordPrefix(suffix, zone) || head == zone || wordPrefix(head, zone)
}

// wordPrefix says whether zone starts with the whole words of short ("redridge" in "redridge mountains", not "red").
func wordPrefix(short, zone string) bool {
	return short != "" && strings.HasPrefix(zone, short+" ")
}

// suffixZone returns the part of the name after the last comma ("auberdine, darkshore" -> "darkshore"); empty without a comma.
func suffixZone(name string) string {
	i := strings.LastIndex(name, ",")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(name[i+1:])
}
