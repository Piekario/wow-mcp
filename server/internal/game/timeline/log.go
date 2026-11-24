// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package timeline

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/eventlog"
	"wowmcp/internal/snapshot"
)

// withLog gives the level-ups and turned-in quests found between dumps the time the addon logged them, and adds the
// logged ones no pair of kept dumps shows (for example older than the oldest kept dump). A level is reached once, so
// a level-up matches the logged levels it spans; a quest matches the latest unused entry of its ID before the dump.
func withLog(events []Event, log []eventlog.Entry) []Event {
	used := make([]bool, len(log))
	for i, e := range events {
		switch what := e.What.(type) {
		case LevelUp:
			for j, l := range log {
				if l.Kind == eventlog.KindLevel && l.Level > what.From && l.Level <= what.To {
					used[j] = true
					if l.Level == what.To {
						events[i].At, events[i].Exact = l.At, true
					}
				}
			}
		case TurnedInQuest:
			id, _ := snapshot.AsInt(what.ID)
			if j := latestTurnIn(log, used, id, e.At); j >= 0 {
				used[j] = true
				events[i].At, events[i].Exact = log[j].At, true
			}
		}
	}
	titles := acceptedTitles(log)
	for j, l := range log {
		if used[j] {
			continue
		}
		switch l.Kind {
		case eventlog.KindLevel:
			events = append(events, Event{At: l.At, Kind: KindLevelUp, What: LevelUp{l.Level - 1, l.Level}, Exact: true})
		case eventlog.KindQuestTurnedIn:
			title, ok := titles[l.QuestID]
			if !ok {
				title = fmt.Sprintf("quest %d", l.QuestID)
			}
			events = append(events, Event{At: l.At, Kind: KindQuest, What: TurnedInQuest{ID: l.QuestID, Title: title}, Exact: true})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })
	return events
}

func latestTurnIn(log []eventlog.Entry, used []bool, questID, before int64) int {
	found := -1
	for j, l := range log {
		if !used[j] && l.Kind == eventlog.KindQuestTurnedIn && l.QuestID == questID && l.At <= before {
			found = j
		}
	}
	return found
}

// acceptedTitles: a turned-in entry has only the quest ID, the entry of its acceptance may have the title.
func acceptedTitles(log []eventlog.Entry) map[int64]string {
	out := map[int64]string{}
	for _, l := range log {
		if l.Kind == eventlog.KindQuestAccepted && l.Title != "" {
			out[l.QuestID] = l.Title
		}
	}
	return out
}
