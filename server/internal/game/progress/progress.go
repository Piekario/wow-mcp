// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package progress covers the character's progress over time: what changed between dumps,
// the XP and gold rate, play sessions and time to the next level. It computes from the dump history,
// reached through the History port (implementation: storage.History).
package progress

import (
	"fmt"
	"time"

	"wowmcp/internal/snapshot"
)

// History is the port to the remembered earlier dumps of a character.
type History interface {
	// List returns the times (dumpedAt) of the character's remembered dumps, oldest first.
	List(character string) ([]int64, error)
	// Load loads the remembered dump of the character from the given moment.
	Load(character string, dumpedAt int64) (snapshot.Snapshot, error)
}

// UnloadedCounter is an optional part of the History port: how many remembered dumps of a character List
// skipped as dumps from an empty client (snapshot.IsUnloadedDump). storage.History implements it.
type UnloadedCounter interface {
	SkippedUnloaded(character string) int
}

// Tracker computes progress from history; a nil History means history is disabled.
type Tracker struct {
	History History
}

// SkippedUnloaded counts the empty-client dumps skipped in the character's history, including the current one.
func (t Tracker) SkippedUnloaded(name string, current snapshot.Snapshot) int {
	n := 0
	if c, ok := t.History.(UnloadedCounter); ok {
		n = c.SkippedUnloaded(name)
	}
	if current.IsUnloadedDump() {
		if at, ok := current.DumpedAt(); !ok || !t.historyHas(name, at) {
			n++ // the current dump is not yet counted in history
		}
	}
	return n
}

// historyHas says whether history already has a dump file from that moment (including one skipped by List).
func (t Tracker) historyHas(name string, at int64) bool {
	if t.History == nil {
		return false
	}
	_, err := t.History.Load(name, at)
	return err == nil
}

// unloadedCurrentNote is the response when the current dump comes from an empty client.
const unloadedCurrentNote = "The current dump comes from exiting the game, when the client no longer had character data (0 gold, empty bags), " +
	"so there is nothing to compare. Log in and type /mcp reload; addon 0.7.1 no longer saves such dumps."

// ChangesQuery is the input of wow_changes.
type ChangesQuery struct {
	Character  string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Steps      int     `json:"steps,omitempty" jsonschema:"which earlier dump to compare with: 1 = the previous one (default), 2 = the one before that, etc."`
	SinceHours float64 `json:"sinceHours,omitempty" jsonschema:"instead of steps: compare with the oldest dump from the last N hours, e.g. 3 = the whole session today"`
}

// changesReport is the comparison asked for in wow_changes. Unloaded: the current dump comes from an empty client;
// a nil Comparison: there is no earlier dump to compare with.
type changesReport struct {
	Unloaded   bool
	History    []int64
	Comparison *Comparison
}

func (t Tracker) changes(character string, current snapshot.Snapshot, q ChangesQuery) (changesReport, error) {
	if t.History == nil {
		return changesReport{}, fmt.Errorf("dump history is disabled (the server was started with --history-dir \"\")")
	}
	times, err := t.History.List(character)
	if err != nil {
		return changesReport{}, err
	}
	if current.IsUnloadedDump() {
		return changesReport{Unloaded: true}, nil
	}
	now, _ := current.DumpedAt()
	var earlier []int64
	for _, at := range times {
		if at < now {
			earlier = append(earlier, at)
		}
	}
	r := changesReport{History: times}
	if len(earlier) == 0 {
		return r, nil
	}
	before, err := t.History.Load(character, baseline(earlier, now, q))
	if err != nil {
		return changesReport{}, err
	}
	c := Compare(before, current)
	r.Comparison = &c
	return r, nil
}

// baseline picks the dump to compare with: the previous one, steps back or the oldest from the last sinceHours.
func baseline(earlier []int64, now int64, q ChangesQuery) int64 {
	base := earlier[len(earlier)-1]
	switch {
	case q.SinceHours > 0:
		cutoff := time.Unix(now, 0).Add(-time.Duration(q.SinceHours * float64(time.Hour))).Unix()
		for _, at := range earlier {
			if at >= cutoff {
				return at
			}
		}
	case q.Steps > 1:
		return earlier[max(len(earlier)-q.Steps, 0)]
	}
	return base
}

// noPoints is the error when there is not a single dump with a time: an old addon or just an empty-client dump.
func noPoints(name string, current snapshot.Snapshot) error {
	if current.IsUnloadedDump() {
		return fmt.Errorf("%s", unloadedCurrentNote)
	}
	return fmt.Errorf("the dump of character %s has no dump time (old addon?). Update the addon and type /mcp reload", name)
}
