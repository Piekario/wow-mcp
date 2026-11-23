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

// Phase says where a zone stands in the route.
type Phase int

const (
	ToDo Phase = iota
	CurrentZone
	Last
)

// Status is where a quest stands.
type Status int

const (
	InProgress Status = iota
	ReadyToTurnIn
	Failed
)

// RouteQuest is one quest of a zone.
type RouteQuest struct {
	Title      string
	Level      int64
	Difficulty string
	Status     Status
	DoLast     bool
}

type Zone struct {
	Name                          string
	MapID                         int64 // UiMapID of the zone from the quests (addon 0.10.0), 0 = unknown
	Quests                        []RouteQuest
	Ready, Doable, TooHard, Total int
	Current                       bool
	Flights                       []flights.Match
}

// Route is the zones in the proposed order. HasLog: the dump has a quests section; HasFlights: it has flightPaths.
type Route struct {
	HasLog      bool
	HasFlights  bool
	PlayerLevel int64
	CurrentZone string
	Zones       []*Zone
	Total       int
	Nodes       []flights.Node
}

// Analyse proposes the zone order.
func Analyse(s snapshot.Snapshot) Route {
	log := quests.ReadLog(s)
	current, currentID := readPosition(s)
	r := Route{HasLog: log.Present, PlayerLevel: log.Level, CurrentZone: current}
	if !log.Present {
		return r
	}
	green, _ := log.EffectiveGreen()
	r.Zones = groupZones(log.Quests, log.Level, green, current, currentID)
	nodes, haveFlights := flights.ReadNodes(s)
	r.HasFlights = haveFlights
	questZones := flights.QuestZones{}
	for _, z := range r.Zones {
		questZones[z.Name] = z.MapID
	}
	r.Nodes = questZones.Guessable(nodes)
	for _, z := range r.Zones {
		z.Flights = flights.MatchZoneID(z.Name, z.MapID, r.Nodes)
		z.sortQuests()
		r.Total += z.Total
	}
	order(r.Zones)
	return r
}

// groupZones groups quests by zone; quests without a zone go to "(no zone)".
func groupZones(list []quests.Quest, level, green int64, current string, currentID int64) []*Zone {
	byName := map[string]*Zone{}
	var zones []*Zone
	for _, q := range list {
		name := q.Zone
		if name == "" {
			name = "(no zone)"
		}
		z := byName[name]
		if z == nil {
			z = &Zone{Name: name, Current: current != "" && strings.EqualFold(name, current)}
			byName[name] = z
			zones = append(zones, z)
		}
		if q.ZoneMapID > 0 && z.MapID == 0 {
			z.MapID = q.ZoneMapID
			// The map ID is the same in every client language, so it identifies the current zone without names.
			z.Current = z.Current || (currentID > 0 && q.ZoneMapID == currentID)
		}
		z.add(q, level, green)
	}
	return zones
}

func (z *Zone) add(q quests.Quest, level, green int64) {
	diff := quests.Difficulty(q.Level, level, green)
	e := RouteQuest{Title: q.Title, Level: q.Level, Difficulty: diff}
	z.Total++
	switch {
	case q.Failed:
		e.Status = Failed
	case q.Complete:
		e.Status = ReadyToTurnIn
		z.Ready++
	case diff == quests.Red:
		e.DoLast = true
		z.TooHard++
	default:
		z.Doable++
	}
	z.Quests = append(z.Quests, e)
}

// onlyTooHard: a zone with nothing to turn in or do, but with red quests.
func (z *Zone) onlyTooHard() bool { return z.Ready == 0 && z.Doable == 0 && z.TooHard > 0 }

func (z *Zone) Phase() Phase {
	switch {
	case z.Current:
		return CurrentZone
	case z.onlyTooHard():
		return Last
	}
	return ToDo
}

// order sorts: current zone, zones with work, zones with only red quests.
func order(zones []*Zone) {
	rank := func(z *Zone) int {
		switch z.Phase() {
		case CurrentZone:
			return 0
		case Last:
			return 2
		}
		return 1
	}
	sort.SliceStable(zones, func(i, j int) bool {
		a, b := zones[i], zones[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		if a.Ready != b.Ready {
			return a.Ready > b.Ready
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.Name < b.Name
	})
}

// sortQuests puts ready quests first, red ones last; within groups from the lowest level.
func (z *Zone) sortQuests() {
	rank := func(q RouteQuest) int {
		switch {
		case q.Status == ReadyToTurnIn:
			return 0
		case q.DoLast:
			return 2
		}
		return 1
	}
	sort.SliceStable(z.Quests, func(i, j int) bool {
		a, b := z.Quests[i], z.Quests[j]
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		return a.Level < b.Level
	})
}
