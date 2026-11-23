// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package leftovers looks in bags and bank for quest items whose quest
// is no longer visible in the log (turned in, abandoned or unknown). These are only
// candidates with an honest confidence rating: the server has no quest database and the
// completedQuests list holds only IDs, so nothing may be removed automatically.
package leftovers

import (
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

// Confidence that the item is a leftover.
const (
	ConfidenceMedium = "medium" // quests in the log do not mention the item
	ConfidenceNone   = "none"   // no quest log: cannot be assessed
	confidenceNA     = "n/a"    // nothing to assess
)

// QuestItem is a quest item summed by name across bags and bank. Item is the first instance encountered.
type QuestItem struct {
	Name           string
	Count          int64
	InBags, InBank bool
	Item           itemkind.Item
}

func (q QuestItem) places() []string {
	var where []string
	if q.InBags {
		where = append(where, "bags")
	}
	if q.InBank {
		where = append(where, "bank")
	}
	return where
}

// Verdict is the assessment of one quest item: needed by quests (Keep) or a leftover candidate.
type Verdict struct {
	Name       string
	Count      int64
	Where      []string
	Keep       bool
	Reason     string
	ForQuests  []string // quests from the log that mention the item
	Confidence string   // only for leftover candidates
}

// Report is every quest item judged against the quest log.
type Report struct {
	Verdicts        []Verdict
	HasLog          bool
	CompletedQuests int
}

// Leftovers are the candidates: quest items no quest in the log needs.
func (r Report) Leftovers() []Verdict {
	var out []Verdict
	for _, v := range r.Verdicts {
		if !v.Keep {
			out = append(out, v)
		}
	}
	return out
}

// Confidence summarises the whole assessment: never "high", because the server has no quest database.
func (r Report) Confidence() string {
	switch {
	case !r.HasLog:
		return ConfidenceNone
	case len(r.Leftovers()) == 0:
		return confidenceNA
	}
	return ConfidenceMedium
}

func Inspect(s snapshot.Snapshot) Report {
	items, log, completed := readQuestItems(s), quests.ReadLog(s), len(s.List("completedQuests"))
	r := Report{HasLog: log.Present, CompletedQuests: completed}
	for _, item := range items {
		r.Verdicts = append(r.Verdicts, judge(item, log))
	}
	return r
}

func judge(item QuestItem, log quests.Log) Verdict {
	v := Verdict{Name: item.Name, Count: item.Count, Where: item.places()}
	refs := questRefs(log, item.Name)
	switch {
	case len(refs) > 0:
		v.Keep, v.ForQuests, v.Reason = true, refs, "mentioned in a quest from the log"
	case item.Item.StartsQuest():
		v.Keep, v.Reason = true, "the tooltip says the item begins a quest, so it may still be useful"
	case !log.Present:
		v.Confidence, v.Reason = ConfidenceNone, "the dump has no quest log, so it cannot be assessed"
	default:
		v.Confidence, v.Reason = ConfidenceMedium, "no quest from the log mentions this item in its title or objectives"
	}
	return v
}

// questRefs finds in the log the quests that mention the item: in the objectives (collect X) or in the title.
func questRefs(log quests.Log, itemName string) []string {
	needle := strings.ToLower(itemName)
	var titles []string
	for _, q := range log.Quests {
		text := strings.ToLower(q.Title + "\n" + strings.Join(q.ObjectiveTexts(), "\n"))
		if needle != "" && strings.Contains(text, needle) {
			titles = append(titles, q.Title)
		}
	}
	return titles
}

// Assess assesses quest items from bags and bank; hasLog = false when the dump has no quest log.
func Assess(s snapshot.Snapshot) (verdicts []Verdict, hasLog bool) {
	r := Inspect(s)
	return r.Verdicts, r.HasLog
}
