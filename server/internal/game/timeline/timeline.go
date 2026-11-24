// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package timeline is the character's timeline: a chronological list of events (level-ups, new spells,
// equipment changes, turned-in quests, deaths, profession milestones) derived from the differences between
// consecutive history dumps. It uses progress.Diff, so it does not repeat the comparison logic.
package timeline

import (
	"fmt"
	"sort"
	"time"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Event kinds (the value of the kind argument and of the kind field in the response).
const (
	KindLevelUp    = "level_up"
	KindNewSpell   = "new_spell"
	KindEquipment  = "equipment"
	KindQuest      = "quest_turned_in"
	KindDeath      = "death"
	KindProfession = "profession"
)

var Kinds = []string{KindLevelUp, KindNewSpell, KindEquipment, KindQuest, KindDeath, KindProfession}

// The default and maximum event limit and how many of the newest history dumps we read.
const (
	defaultLimit = 50
	maxLimit     = 500
	maxSnapshots = 400
)

// professionMilestones are the profession skill thresholds (classic WoW ranks) we treat as milestones.
var professionMilestones = []int64{75, 150, 225, 300}

// Query is the input of the timeline tool.
type Query struct {
	Limit int     `json:"limit,omitempty" jsonschema:"how many of the newest events to show; default 50, max 500"`
	Kind  string  `json:"kind,omitempty" jsonschema:"only one kind: level_up, new_spell, equipment, quest_turned_in, death, profession"`
	Days  float64 `json:"days,omitempty" jsonschema:"only events from the last N days (counted from the time of the last dump)"`
}

// Event is one event on the timeline; What holds the facts of its kind.
type Event struct {
	At    int64
	Kind  string
	What  fact
	Exact bool // the time comes from the addon's event log, not from a dump
}

type LevelUp struct{ From, To int64 }

type NewSpell struct {
	Name    string
	Rank    any
	HasRank bool
}

// GearChange names are as the dump holds them; an empty one is an empty slot.
type GearChange struct{ Slot, Before, After string }

type TurnedInQuest struct {
	ID, Title any
	Zone      string
}

// ProfessionStep is a new profession (New) or crossing a Milestone.
type ProfessionStep struct {
	Name                     string
	Before, After, Milestone int64
	New                      bool
}

// Death keeps the place as the addon recorded it; Known is false when the recent list did not cover the death.
type Death struct {
	Known bool
	recentDeath
}

// report is the timeline after the filters; Events holds at most the limit, the newest.
type report struct {
	SnapshotsRead int
	Events        []Event
	Total         int
	OlderNotShown int
	LogUsed       bool   // a long-kept event log was asked for exact times
	LogError      string // it failed: every event has the time of a dump
}

func build(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (report, error) {
	limit, err := q.limit()
	if err != nil {
		return report{}, err
	}
	if tr.History == nil {
		return report{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the timeline is built from it")
	}
	snaps, err := load(name, current, tr.History)
	if err != nil {
		return report{}, err
	}
	r := report{SnapshotsRead: len(snaps)}
	var events []Event
	if len(snaps) >= 2 {
		events = Build(snaps)
	}
	newest := dumpedAt(current)
	if len(snaps) > 0 {
		newest = dumpedAt(snaps[len(snaps)-1])
	}
	if tr.Log != nil {
		r.LogUsed = true
		if entries, err := tr.Log.Events(name, 0, newest); err != nil {
			r.LogError = err.Error()
		} else {
			events = withLog(events, entries)
		}
	}
	events = filter(events, q, newest)
	r.Total = len(events)
	if len(events) > limit {
		r.OlderNotShown = len(events) - limit
		events = events[len(events)-limit:]
	}
	r.Events = events
	return r, nil
}

func (q Query) limit() (int, error) {
	if q.Kind != "" && !validKind(q.Kind) {
		return 0, fmt.Errorf("unknown kind %q; allowed: %v", q.Kind, Kinds)
	}
	if q.Days < 0 {
		return 0, fmt.Errorf("days cannot be negative, got %v", q.Days)
	}
	switch {
	case q.Limit == 0:
		return defaultLimit, nil
	case q.Limit < 0 || q.Limit > maxLimit:
		return 0, fmt.Errorf("limit must be from 1 to %d (default %d), got %d", maxLimit, defaultLimit, q.Limit)
	}
	return q.Limit, nil
}

func validKind(k string) bool {
	for _, v := range Kinds {
		if v == k {
			return true
		}
	}
	return false
}

func filter(events []Event, q Query, newest int64) []Event {
	var cutoff int64
	if q.Days > 0 {
		cutoff = newest - int64(q.Days*float64(24*time.Hour/time.Second))
	}
	var out []Event
	for _, e := range events {
		if (q.Kind == "" || e.Kind == q.Kind) && e.At >= cutoff {
			out = append(out, e)
		}
	}
	return out
}

// Build turns consecutive dumps (oldest first) into events sorted by time.
func Build(snaps []snapshot.Snapshot) []Event {
	var events []Event
	for i := 1; i < len(snaps); i++ {
		events = append(events, between(snaps[i-1], snaps[i])...)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })
	return events
}

// between extracts events from the comparison of two dumps (progress.Compare).
func between(before, after snapshot.Snapshot) []Event {
	at, ok := after.DumpedAt()
	if !ok {
		return nil
	}
	c := progress.Compare(before, after)
	var events []Event
	if c.XP != nil && c.XP.LevelUp() {
		events = append(events, Event{At: at, Kind: KindLevelUp, What: LevelUp{c.XP.LevelBefore, c.XP.LevelAfter}})
	}
	for _, s := range c.NewSpells {
		if name, _ := s.Name.(string); name != "" {
			events = append(events, Event{At: at, Kind: KindNewSpell, What: NewSpell{name, s.Rank, s.HasRank && s.Rank != nil}})
		}
	}
	for _, g := range c.Equipment {
		events = append(events, Event{At: at, Kind: KindEquipment, What: GearChange{g.Slot, g.Before, g.After}})
	}
	for _, q := range c.Quests.TurnedIn {
		zone, _ := q.Zone.(string)
		events = append(events, Event{At: at, Kind: KindQuest, What: TurnedInQuest{q.ID, q.Title, zone}})
	}
	events = append(events, professions(c.Professions, at)...)
	if c.Deaths != nil {
		events = append(events, deaths(before, after, c.Deaths.Delta(), at)...)
	}
	return events
}

// professions reports a new profession and crossing the 75/150/225/300 threshold (classic WoW ranks).
func professions(changes []progress.SkillChange, at int64) []Event {
	var out []Event
	for _, p := range changes {
		if p.New {
			out = append(out, Event{At: at, Kind: KindProfession, What: ProfessionStep{Name: p.Name, After: p.After, New: true}})
		} else if m, ok := crossed(p.Before, p.After); ok {
			out = append(out, Event{At: at, Kind: KindProfession, What: ProfessionStep{Name: p.Name, Before: p.Before, After: p.After, Milestone: m}})
		}
	}
	return out
}

func crossed(before, after int64) (int64, bool) {
	var found int64
	for _, m := range professionMilestones {
		if before < m && after >= m {
			found = m
		}
	}
	return found, found > 0
}

// deaths creates one event per new death; the time comes from the recent list or, when missing, from the dump time.
func deaths(before, after snapshot.Snapshot, n, at int64) []Event {
	from, _ := before.DumpedAt()
	var out []Event
	for _, r := range readRecentDeaths(after) {
		if !r.HasAt || r.At <= from || r.At > at {
			continue
		}
		out = append(out, Event{At: r.At, Kind: KindDeath, What: Death{true, r}})
	}
	if int64(len(out)) > n {
		out = out[int64(len(out))-n:]
	}
	for int64(len(out)) < n {
		out = append(out, Event{At: at, Kind: KindDeath, What: Death{}})
	}
	return out
}
