// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeons

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// View is the answer of wow_dungeons.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
	return Analyse(s, q, fresh).Render(s)
}

func (r Report) Render(s snapshot.Snapshot) map[string]any {
	out := map[string]any{"level": r.Level, "fitNow": renderEntries(r.Fit)}
	if r.Source == FromTable {
		out["source"] = "classic"
		out["sourceNote"] = "The client did not report a dungeon list, so the ranges come from the classic game table and are approximate; Forever may change them. If the game has a group finder tool, open it once and type /mcp reload."
	} else {
		out["source"] = "game"
		out["sourceNote"] = "The level ranges come from the game client (the group finder tool)."
	}
	if len(r.Soon) > 0 {
		out["soon"] = renderEntries(r.Soon)
	}
	if len(r.Other) > 0 {
		out["other"] = renderEntries(r.Other)
		if !r.All {
			out["otherNote"] = "Dungeons outside your level range that you have quests for in the log."
		}
	}
	if len(r.Lockouts) > 0 {
		out["lockouts"] = renderLockouts(r.Lockouts)
	}
	if r.Source == FromTable {
		s.AddEnglishOnlyNote(out, "The dungeon list, zones, notes and the names used to find dungeon quests in the quest log come from a table of English dungeon names, because the client gave no dungeon list")
	} else if r.WithoutTable > 0 {
		s.AddEnglishOnlyNote(out, fmt.Sprintf("Levels come from the client, but the zone and notes of a dungeon come from a table of English dungeon names, so they are missing for %d dungeons", r.WithoutTable))
	}
	return out
}

// renderEntries is never nil, so JSON has [] instead of null.
func renderEntries(entries []Entry) []any {
	out := []any{}
	for _, e := range entries {
		out = append(out, e.render())
	}
	return out
}

func (e Entry) render() map[string]any {
	m := map[string]any{"name": e.Name}
	if e.ID > 0 {
		m["id"] = e.ID
	}
	if e.MapID > 0 {
		m["mapID"] = e.MapID
		if e.Locked {
			m["locked"] = true
		}
	}
	if e.Min > 0 {
		m["levels"] = fmt.Sprintf("%d-%d", e.Min, e.Max)
	}
	if e.Zone != "" {
		m["zone"] = e.Zone
	}
	if e.Note != "" {
		m["note"] = e.Note
	}
	if e.Raid {
		m["raid"] = true
	}
	if e.Players > 0 {
		m["players"] = e.Players
	}
	if len(e.Quests) > 0 {
		quests := make([]any, len(e.Quests))
		for i, q := range e.Quests {
			quests[i] = q.render()
		}
		m["quests"] = quests
	}
	if e.LevelsToGo > 0 {
		m["levelsToGo"] = e.LevelsToGo
	}
	return m
}

func (r RelatedQuest) render() map[string]any {
	q := r.Quest
	m := map[string]any{"title": q.Title, "id": q.ID, "level": nil}
	if q.HasLevel {
		m["level"] = q.Level
	}
	if r.HasDifficulty {
		m["difficulty"] = r.Difficulty
	}
	if q.Complete {
		m["complete"] = true
	} else if goals := q.ObjectiveTexts(); len(goals) > 0 {
		m["objectives"] = strings.Join(goals, "; ")
	}
	return m
}

func renderLockouts(lockouts []LockoutStatus) []any {
	out := []any{}
	for _, l := range lockouts {
		e := map[string]any{"name": l.Name}
		passed := map[string]any{"difficulty": l.Difficulty, "difficultyID": l.DifficultyID, "instanceID": l.InstanceID, "raid": l.Raid, "maxPlayers": l.MaxPlayers}
		for k, v := range passed {
			if v != nil {
				e[k] = v
			}
		}
		if l.HasKilled {
			e["progress"] = fmt.Sprintf("%d/%d", l.Killed, l.Encounters)
		}
		if l.HasReset {
			if l.Passed {
				e["reset"] = "already reset"
			} else {
				e["resetIn"] = snapshot.FormatAge(l.Left)
			}
		}
		out = append(out, e)
	}
	return out
}
