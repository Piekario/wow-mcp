// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package timeline

import (
	"fmt"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// fact is what an event says: the sentence and its details for the answer.
type fact interface {
	describe() (text string, details map[string]any)
}

// View builds the character's timeline from history and the current dump.
func View(name string, current snapshot.Snapshot, tr progress.Tracker, q Query) (map[string]any, error) {
	r, err := build(name, current, tr, q)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"order": "from the oldest to the newest event (with a limit the newest are kept)",
		"assumption": "an event gets the time of the first dump in which it is visible (the game saves data only on /reload and logout), " +
			"so the real time may be earlier; the exception is deaths, which the addon saves with an exact time",
		"snapshotsRead": r.SnapshotsRead,
	}
	if r.LogUsed {
		out["assumption"] = "an event gets the time of the first dump in which it is visible (the game saves data only on /reload and logout), " +
			"so the real time may be earlier; events with exact: true (level-ups and turned-in quests) have the time from the addon's event log, " +
			"which also reaches back before the kept dumps, and deaths are saved with an exact time too"
	}
	if r.LogError != "" {
		out["eventLogError"] = "the event log could not be read, so every event has the time of a dump: " + r.LogError
	}
	tr.NoteSkipped(out, name, current)
	if r.SnapshotsRead < 2 && len(r.Events) == 0 {
		out["events"] = []any{}
		out["note"] = "The history has one dump, so there is nothing to compare with. Events will appear after the next /mcp reload in game."
		return out, nil
	}
	out["total"] = r.Total
	if r.OlderNotShown > 0 {
		out["olderNotShown"] = r.OlderNotShown
	}
	list := make([]any, len(r.Events))
	for i, e := range r.Events {
		list[i] = e.render()
	}
	out["events"] = list
	return out, nil
}

func (e Event) render() map[string]any {
	text, details := e.What.describe()
	out := map[string]any{"at": e.At, "atText": snapshot.FormatTime(e.At), "kind": e.Kind, "text": text}
	if e.Exact {
		out["exact"] = true
	}
	if len(details) > 0 {
		out["details"] = details
	}
	return out
}

func (l LevelUp) describe() (string, map[string]any) {
	return fmt.Sprintf("Level up: level %d -> %d", l.From, l.To), map[string]any{"levelBefore": l.From, "levelAfter": l.To}
}

func (s NewSpell) describe() (string, map[string]any) {
	text, details := "New spell: "+s.Name, map[string]any{"name": s.Name}
	if s.HasRank {
		text += fmt.Sprintf(" (%v)", s.Rank)
		details["rank"] = s.Rank
	}
	return text, details
}

func (g GearChange) describe() (string, map[string]any) {
	before, after := snapshot.OrEmpty(g.Before), snapshot.OrEmpty(g.After)
	return fmt.Sprintf("Equipment, slot %s: %s -> %s", g.Slot, before, after), map[string]any{"slot": g.Slot, "before": before, "after": after}
}

func (q TurnedInQuest) describe() (string, map[string]any) {
	details := map[string]any{"id": q.ID}
	text := fmt.Sprintf("Quest turned in: %v", q.Title)
	if q.Zone != "" {
		details["zone"] = q.Zone
		text += " (" + q.Zone + ")"
	}
	return text, details
}

func (p ProfessionStep) describe() (string, map[string]any) {
	details := map[string]any{"name": p.Name, "after": p.After}
	if p.New {
		details["note"] = "new"
		return fmt.Sprintf("New profession or skill: %s (%d)", p.Name, p.After), details
	}
	details["before"], details["milestone"] = p.Before, p.Milestone
	return fmt.Sprintf("%s: %d -> %d (milestone %d)", p.Name, p.Before, p.After, p.Milestone), details
}

func (d Death) describe() (string, map[string]any) {
	details := map[string]any{}
	if d.HasZone {
		details["zone"] = d.Zone
	}
	if d.HasSubZone {
		details["subZone"] = d.SubZone
	}
	if d.HasLevel {
		details["level"] = d.Level
	}
	text := "Death"
	if zone, ok := d.Zone.(string); ok && zone != "" {
		text += ": " + zone
	}
	return text, details
}
