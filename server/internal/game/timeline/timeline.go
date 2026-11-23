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

// Event is one event on the timeline.
type Event struct {
	At      int64
	Kind    string
	Text    string
	Details map[string]any
}

func (e Event) view() map[string]any {
	out := map[string]any{"at": e.At, "atText": snapshot.FormatTime(e.At), "kind": e.Kind, "text": e.Text}
	if len(e.Details) > 0 {
		out["details"] = e.Details
	}
	return out
}

// View builds the character's timeline from history and the current dump.
func View(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	limit, err := q.limit()
	if err != nil {
		return nil, err
	}
	if tr.History == nil {
		return nil, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\"), and the timeline is built from it")
	}
	snaps, err := load(name, current, tr.History)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"order": "from the oldest to the newest event (with a limit the newest are kept)",
		"assumption": "an event gets the time of the first dump in which it is visible (the game saves data only on /reload and logout), " +
			"so the real time may be earlier; the exception is deaths, which the addon saves with an exact time",
		"snapshotsRead": len(snaps),
	}
	tr.NoteSkipped(out, name, current)
	if len(snaps) < 2 {
		out["events"] = []any{}
		out["note"] = "The history has one dump, so there is nothing to compare with. Events will appear after the next /mcp reload in game."
		return out, nil
	}
	events := Build(snaps)
	events = filter(events, q, snaps[len(snaps)-1])
	out["total"] = len(events)
	if len(events) > limit {
		out["olderNotShown"] = len(events) - limit
		events = events[len(events)-limit:]
	}
	list := make([]any, len(events))
	for i, e := range events {
		list[i] = e.view()
	}
	out["events"] = list
	return out, nil
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

// load reads the newest history dumps (oldest first) and appends the current one if it is newer than the last.
func load(name string, current snapshot.Snapshot, h progress.History) ([]snapshot.Snapshot, error) {
	times, err := h.List(name)
	if err != nil {
		return nil, err
	}
	if len(times) > maxSnapshots {
		times = times[len(times)-maxSnapshots:]
	}
	var snaps []snapshot.Snapshot
	for _, at := range times {
		if s, err := h.Load(name, at); err == nil { // a corrupt history file does not spoil the timeline
			snaps = append(snaps, s)
		}
	}
	cur, ok := current.DumpedAt()
	// A dump from an empty client (exit from the game) does not describe the character: we do not append it.
	if last := len(snaps) - 1; ok && !current.IsUnloadedDump() && (last < 0 || dumpedAt(snaps[last]) < cur) {
		snaps = append(snaps, current)
	}
	return snaps, nil
}

func dumpedAt(s snapshot.Snapshot) int64 {
	at, _ := s.DumpedAt()
	return at
}

func filter(events []Event, q Query, newest snapshot.Snapshot) []Event {
	var cutoff int64
	if q.Days > 0 {
		cutoff = dumpedAt(newest) - int64(q.Days*float64(24*time.Hour/time.Second))
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

// between extracts events from the difference of two dumps (progress.Diff).
func between(before, after snapshot.Snapshot) []Event {
	at, ok := after.DumpedAt()
	if !ok {
		return nil
	}
	d := progress.Diff(before, after)
	var events []Event
	events = append(events, levelUp(d, at)...)
	events = append(events, spells(d, at)...)
	events = append(events, equipment(d, at)...)
	events = append(events, quests(d, at)...)
	events = append(events, professions(d, at)...)
	events = append(events, deaths(before, after, d, at)...)
	return events
}

func levelUp(d map[string]any, at int64) []Event {
	xp := snapshot.AsMap(d["experience"])
	if xp["levelUp"] != true {
		return nil
	}
	from, _ := snapshot.AsInt(xp["levelBefore"])
	to, _ := snapshot.AsInt(xp["levelAfter"])
	return []Event{{At: at, Kind: KindLevelUp, Text: fmt.Sprintf("Level up: level %d -> %d", from, to),
		Details: map[string]any{"levelBefore": from, "levelAfter": to}}}
}

func spells(d map[string]any, at int64) []Event {
	var out []Event
	for _, raw := range snapshot.AsList(d["newSpells"]) {
		s := snapshot.AsMap(raw)
		name, _ := s["name"].(string)
		if name == "" {
			continue
		}
		text := "New spell: " + name
		details := map[string]any{"name": name}
		if rank, ok := s["rank"]; ok && rank != nil {
			text += fmt.Sprintf(" (%v)", rank)
			details["rank"] = rank
		}
		out = append(out, Event{At: at, Kind: KindNewSpell, Text: text, Details: details})
	}
	return out
}

func equipment(d map[string]any, at int64) []Event {
	var out []Event
	for _, raw := range snapshot.AsList(d["equipment"]) {
		e := snapshot.AsMap(raw)
		slot, _ := e["slot"].(string)
		before, after := fmt.Sprint(e["before"]), fmt.Sprint(e["after"])
		out = append(out, Event{At: at, Kind: KindEquipment, Text: fmt.Sprintf("Equipment, slot %s: %s -> %s", slot, before, after),
			Details: map[string]any{"slot": slot, "before": before, "after": after}})
	}
	return out
}

func quests(d map[string]any, at int64) []Event {
	var out []Event
	for _, raw := range snapshot.AsList(snapshot.AsMap(d["quests"])["turnedIn"]) {
		q := snapshot.AsMap(raw)
		details := map[string]any{"id": q["id"]}
		text := fmt.Sprintf("Quest turned in: %v", q["title"])
		if zone, ok := q["zone"].(string); ok && zone != "" {
			details["zone"] = zone
			text += " (" + zone + ")"
		}
		out = append(out, Event{At: at, Kind: KindQuest, Text: text, Details: details})
	}
	return out
}

// professions reports a new profession and crossing the 75/150/225/300 threshold (classic WoW ranks).
func professions(d map[string]any, at int64) []Event {
	var out []Event
	for _, raw := range snapshot.AsList(d["professions"]) {
		p := snapshot.AsMap(raw)
		name, _ := p["name"].(string)
		after, _ := snapshot.AsInt(p["after"])
		before, hadBefore := snapshot.AsInt(p["before"])
		details := map[string]any{"name": name, "after": after}
		switch {
		case !hadBefore:
			details["note"] = "new"
			out = append(out, Event{At: at, Kind: KindProfession, Text: fmt.Sprintf("New profession or skill: %s (%d)", name, after), Details: details})
		default:
			if m, ok := crossed(before, after); ok {
				details["before"], details["milestone"] = before, m
				out = append(out, Event{At: at, Kind: KindProfession, Text: fmt.Sprintf("%s: %d -> %d (milestone %d)", name, before, after, m), Details: details})
			}
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
func deaths(before, after snapshot.Snapshot, d map[string]any, at int64) []Event {
	n, _ := snapshot.AsInt(snapshot.AsMap(d["deaths"])["new"])
	if n <= 0 {
		return nil
	}
	from, _ := before.DumpedAt()
	var out []Event
	for _, raw := range snapshot.AsList(after.Map("deaths")["recent"]) {
		r := snapshot.AsMap(raw)
		when, ok := snapshot.AsInt(r["at"])
		if !ok || when <= from || when > at {
			continue
		}
		out = append(out, deathEvent(when, r))
	}
	if int64(len(out)) > n {
		out = out[int64(len(out))-n:]
	}
	for int64(len(out)) < n {
		out = append(out, deathEvent(at, nil))
	}
	return out
}

func deathEvent(at int64, r map[string]any) Event {
	details := map[string]any{}
	text := "Death"
	for _, key := range []string{"zone", "subZone", "level"} {
		if v, ok := r[key]; ok {
			details[key] = v
		}
	}
	if zone, ok := r["zone"].(string); ok && zone != "" {
		text += ": " + zone
	}
	return Event{At: at, Kind: KindDeath, Text: text, Details: details}
}
