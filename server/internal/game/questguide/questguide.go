// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package questguide answers "where do I pick up, do and turn in this quest" and "what is my next guide step".
//
// The server holds no quest data of its own. Everything comes from the dump: the questDb section (where quests
// start and end, read by the addon from the quest database the player has installed, QuestieDB), the
// questLookups section (answers to /mcp quest in game) and the guide section (the next steps of the player's
// Guidelime guide, e.g. TUGs). The quest text in the game is often out of date and World of Warcraft: Forever
// differs from Classic, so when the dump has no data the answer says how to get it instead of guessing.
package questguide

import (
	"math"
	"strconv"
	"strings"

	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_quest_lookup.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = most recently dumped character"`
	Quest     string `json:"quest,omitempty" jsonschema:"quest ID, or a fragment of a quest or NPC name; empty = every quest in the log"`
}

// Where a quest stands for the character.
const (
	statusReady     = "in log, ready to turn in"
	statusActive    = "in log, in progress"
	statusFailed    = "in log, failed"
	statusCompleted = "completed"
	statusNotInLog  = "not in log"
)

// Player is where the dump says the character stands. Known: zone and coordinates are present.
type Player struct {
	Zone  string
	X, Y  float64
	Known bool
}

// Location is one spawn point from the quest database.
type Location struct {
	Zone       string
	AreaID     int64
	HasAreaID  bool
	InInstance bool
	X, Y       float64
	HasXY      bool
}

// Target is an NPC, object or item of the quest database. The fields the answer only passes through stay as the
// dump has them (nil = not in the dump).
type Target struct {
	Kind, ID, Name, Title, Text, FriendlyTo, SpawnCount, Zones, ZoneCount any
	Locations                                                             []Location
}

// Entry is a quest of the quest database.
type Entry struct {
	ID, Name, Level, RequiredLevel, Zone, ObjectivesText, NextQuest, InGuide any
	InLog                                                                    bool
	StartedBy, FinishedBy, Objectives                                        []Target
}

// SavedLookup is the answer to one /mcp quest the player asked in game.
type SavedLookup struct {
	Query  string
	Quests []Entry
	NPCs   []Target
}

// Database is the questDb section and the saved /mcp quest answers.
type Database struct {
	Present bool
	Source  any
	Quests  []Entry
	Unknown []any
	Lookups []SavedLookup
}

// compass names the direction of a map offset (y grows to the south).
func compass(dx, dy float64) string {
	names := []string{"east", "south-east", "south", "south-west", "west", "north-west", "north", "north-east"}
	angle := math.Atan2(dy, dx) // 0 = east, positive = clockwise on the map
	index := int(math.Round(angle/(math.Pi/4))+8) % 8
	return names[index]
}

// Spot is a location, with the distance from the character when both are in the same zone.
type Spot struct {
	Location
	Distance    float64
	HasDistance bool
	Direction   string
}

// PlaceNote is what the answer must say about a target without (all) its locations.
type PlaceNote int

const (
	NoNote PlaceNote = iota
	ItemWithoutDrop
	NotInDatabase
	MoreSpawns
)

// Place is a target with its spots.
type Place struct {
	Target Target
	Spots  []Spot
	Note   PlaceNote
	Spawns int64
}

func place(t Target, p Player) Place {
	out := Place{Target: t}
	if len(t.Locations) == 0 {
		if t.Kind == "item" {
			out.Note = ItemWithoutDrop
		} else if t.Name == nil {
			out.Note = NotInDatabase
		}
		return out
	}
	for _, loc := range t.Locations {
		spot := Spot{Location: loc}
		if loc.HasXY && p.Known && loc.Zone == p.Zone {
			dx, dy := loc.X-p.X, loc.Y-p.Y
			spot.Distance, spot.HasDistance, spot.Direction = snapshot.Round1(math.Hypot(dx, dy)), true, compass(dx, dy)
		}
		out.Spots = append(out.Spots, spot)
	}
	if count, _ := snapshot.AsInt(t.SpawnCount); count > int64(len(out.Spots)) {
		out.Note, out.Spawns = MoreSpawns, count
	}
	return out
}

func places(targets []Target, p Player) []Place {
	var out []Place
	for _, t := range targets {
		out = append(out, place(t, p))
	}
	return out
}

// logStates maps quest ID -> state for the quests in the log and the completed ones.
func logStates(s snapshot.Snapshot) map[int64]string {
	states := map[int64]string{}
	for _, id := range readCompleted(s) {
		states[id] = statusCompleted
	}
	for _, q := range quests.ReadLog(s).Quests {
		id, ok := snapshot.AsInt(q.ID)
		if !ok {
			continue
		}
		switch {
		case q.Failed:
			states[id] = statusFailed
		case q.Complete:
			states[id] = statusReady
		default:
			states[id] = statusActive
		}
	}
	return states
}

func statusOf(states map[int64]string, id int64) string {
	if state, ok := states[id]; ok {
		return state
	}
	return statusNotInLog
}

// NextAction is the one thing the player should do about a quest.
type NextAction int

const (
	NoAction NextAction = iota
	TurnIn
	TurnInUnknownPlace
	FinishThenTurnIn
	PickUp
	AbandonAndRetake
)

// nextAction is the answer for a quest in the given state; At is the target the action points to.
func nextAction(e Entry, status string) (action NextAction, at *Target) {
	first := func(targets []Target) *Target {
		if len(targets) == 0 {
			return nil
		}
		return &targets[0]
	}
	switch status {
	case statusReady:
		if at = first(e.FinishedBy); at != nil {
			return TurnIn, at
		}
		return TurnInUnknownPlace, nil
	case statusActive:
		if at = first(e.FinishedBy); at != nil {
			return FinishThenTurnIn, at
		}
	case statusNotInLog:
		if at = first(e.StartedBy); at != nil {
			return PickUp, at
		}
	case statusFailed:
		return AbandonAndRetake, nil
	}
	return NoAction, nil
}

// QuestAnswer is one quest of the answer.
type QuestAnswer struct {
	Entry                      Entry
	Status                     string
	Next                       NextAction
	At                         *Target
	TurnIn, Objectives, PickUp []Place
}

func answerFor(e Entry, states map[int64]string, p Player) QuestAnswer {
	id, _ := snapshot.AsInt(e.ID)
	a := QuestAnswer{Entry: e, Status: statusOf(states, id)}
	a.Next, a.At = nextAction(e, a.Status)
	if a.Status != statusCompleted {
		a.TurnIn = places(e.FinishedBy, p)
		if a.Status != statusReady {
			a.Objectives = places(e.Objectives, p)
		}
	}
	if a.Status == statusNotInLog || a.Status == statusFailed {
		a.PickUp = places(e.StartedBy, p)
	}
	return a
}

// matches reports whether a quest or NPC fits the query (an ID, or a name fragment in any letter case).
func matches(id, name any, query string) bool {
	if query == "" {
		return true
	}
	if want, err := strconv.ParseInt(query, 10, 64); err == nil {
		got, ok := snapshot.AsInt(id)
		return ok && got == want
	}
	text, _ := name.(string)
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

// LookupResult is the quests and NPCs that fit the query. Available is false when the dump has no quest database.
type LookupResult struct {
	Available bool
	Query     string
	Player    Player
	Source    any
	Quests    []QuestAnswer
	NPCs      []Place
	Unknown   []any
}

type finder struct {
	query   string
	states  map[int64]string
	player  Player
	seen    map[int64]bool
	seenNPC map[int64]bool
	result  *LookupResult
}

func (f *finder) addQuest(e Entry, requireLog bool) {
	id, ok := snapshot.AsInt(e.ID)
	if !ok || f.seen[id] || !matches(e.ID, e.Name, f.query) || (requireLog && !e.InLog) {
		return
	}
	f.seen[id] = true
	f.result.Quests = append(f.result.Quests, answerFor(e, f.states, f.player))
}

// addSaved takes one answer to /mcp quest. When the player asked this very question every quest of the answer counts.
func (f *finder) addSaved(l SavedLookup) {
	same := strings.EqualFold(strings.TrimSpace(l.Query), f.query)
	for _, e := range l.Quests {
		if !same {
			f.addQuest(e, false)
		} else if id, ok := snapshot.AsInt(e.ID); ok && !f.seen[id] {
			f.seen[id] = true
			f.result.Quests = append(f.result.Quests, answerFor(e, f.states, f.player))
		}
	}
	for _, npc := range l.NPCs {
		id, ok := snapshot.AsInt(npc.ID)
		if !ok || f.seenNPC[id] || !(same || matches(npc.ID, npc.Name, f.query)) {
			continue
		}
		f.seenNPC[id] = true
		f.result.NPCs = append(f.result.NPCs, place(npc, f.player))
	}
}

// Find looks for the matching quests: where they start, what they need and where they end.
func Find(s snapshot.Snapshot, q Query) LookupResult {
	query := strings.TrimSpace(q.Quest)
	db := readDatabase(s)
	r := LookupResult{Query: query}
	if !db.Present && len(db.Lookups) == 0 {
		return r
	}
	r.Available, r.Source, r.Player = true, db.Source, readPlayer(s)
	f := &finder{query: query, states: logStates(s), player: r.Player, seen: map[int64]bool{}, seenNPC: map[int64]bool{}, result: &r}
	// Without a query: the quest log. With one: also the guide quests and the answers to /mcp quest.
	for _, e := range db.Quests {
		f.addQuest(e, query == "")
	}
	if query != "" {
		for _, l := range db.Lookups {
			f.addSaved(l)
		}
	}
	if query == "" {
		r.Unknown = db.Unknown
	}
	return r
}
