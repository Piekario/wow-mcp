// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package leftovers looks in bags and bank for quest items whose quest
// is no longer visible in the log (turned in, abandoned or unknown). These are only
// candidates with an honest confidence rating: the server has no quest database and the
// completedQuests list holds only IDs, so nothing may be removed automatically.
package leftovers

import (
	"sort"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

// Confidence that the item is a leftover.
const (
	ConfidenceMedium = "medium" // quests in the log do not mention the item
	ConfidenceNone   = "none"   // no quest log: cannot be assessed
)

// stack is a quest item summed by name across bags and bank.
type stack struct {
	name  string
	count int64
	where map[string]bool
	item  map[string]any // first instance encountered (for the tooltip)
}

// questItems collects quest items from bags and bank, by name, in a fixed order.
func questItems(s snapshot.Snapshot) []*stack {
	byName := map[string]*stack{}
	for _, source := range []string{"bags", "bank"} {
		for _, f := range items.Find(s, "", source) {
			if !itemkind.IsQuestItem(f.Item) {
				continue
			}
			name, _ := f.Item["name"].(string)
			st := byName[name]
			if st == nil {
				st = &stack{name: name, where: map[string]bool{}, item: f.Item}
				byName[name] = st
			}
			st.count += snapshot.ItemCount(f.Item)
			st.where[source] = true
		}
	}
	list := make([]*stack, 0, len(byName))
	for _, st := range byName {
		list = append(list, st)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}

func (st *stack) entry() map[string]any {
	var where []string
	for _, w := range []string{"bags", "bank"} {
		if st.where[w] {
			where = append(where, w)
		}
	}
	return map[string]any{"name": st.name, "count": st.count, "where": where}
}

// questRefs finds in the log the quests that mention the item: in the objectives (collect X) or in the title.
func questRefs(log []any, itemName string) []string {
	needle := strings.ToLower(itemName)
	var titles []string
	for _, raw := range log {
		q := snapshot.AsMap(raw)
		title, _ := q["title"].(string)
		text := strings.ToLower(title + "\n" + strings.Join(quests.Objectives(q), "\n"))
		if needle != "" && strings.Contains(text, needle) {
			titles = append(titles, title)
		}
	}
	return titles
}

// beginsQuest: the item begins a quest (questID from the container, or the English tooltip line), so it may still be needed.
func beginsQuest(item map[string]any) bool { return itemkind.StartsQuest(item) }

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

func (v Verdict) entry() map[string]any {
	e := map[string]any{"name": v.Name, "count": v.Count, "where": v.Where}
	if len(v.ForQuests) > 0 {
		e["forQuests"] = v.ForQuests
	}
	e["reason"] = v.Reason
	if !v.Keep {
		e["confidence"] = v.Confidence
	}
	return e
}

// Assess assesses quest items from bags and bank; hasLog = false when the dump has no quest log.
func Assess(s snapshot.Snapshot) (verdicts []Verdict, hasLog bool) {
	log, hasLog := s["quests"].([]any)
	for _, st := range questItems(s) {
		e := st.entry()
		v := Verdict{Name: st.name, Count: st.count, Where: e["where"].([]string)}
		refs := questRefs(log, st.name)
		switch {
		case len(refs) > 0:
			v.Keep, v.ForQuests, v.Reason = true, refs, "mentioned in a quest from the log"
		case beginsQuest(st.item):
			v.Keep, v.Reason = true, "the tooltip says the item begins a quest, so it may still be useful"
		case !hasLog:
			v.Confidence, v.Reason = ConfidenceNone, "the dump has no quest log, so it cannot be assessed"
		default:
			v.Confidence, v.Reason = ConfidenceMedium, "no quest from the log mentions this item in its title or objectives"
		}
		verdicts = append(verdicts, v)
	}
	return verdicts, hasLog
}

// View assesses quest items: which are needed by quests in the log (keep) and which
// look like leftovers (leftovers), together with the confidence of the assessment.
func View(s snapshot.Snapshot) map[string]any {
	verdicts, hasLog := Assess(s)
	if len(verdicts) == 0 {
		return map[string]any{
			"questItems": 0,
			"note":       "No quest items (type Quest) in bags and bank. The bank shows the state from the last visit.",
		}
	}
	keep, leftovers := []any{}, []any{}
	for _, v := range verdicts {
		if v.Keep {
			keep = append(keep, v.entry())
		} else {
			leftovers = append(leftovers, v.entry())
		}
	}
	out := map[string]any{
		"questItems": len(verdicts),
		"keep":       keep,
		"leftovers":  leftovers,
		"confidence": overallConfidence(hasLog, len(leftovers)),
		"advice":     "These are only candidates. The server does not know whether the item is needed for a quest not yet taken or later in a chain. Before discarding, check the item tooltip in game; nothing is removed and nothing should be removed automatically.",
		"limits":     "Matching is by item name against quest titles and objectives from the log. completedQuests contains only IDs, without names, so an item cannot be tied to a turned-in quest. The bank shows the state from the last visit.",
	}
	if n := len(s.List("completedQuests")); n > 0 {
		out["completedQuestsKnown"] = n
	}
	return out
}

// overallConfidence summarises the confidence of the whole assessment: never "high", because the server has no quest database.
func overallConfidence(hasLog bool, candidates int) string {
	switch {
	case !hasLog:
		return ConfidenceNone
	case candidates == 0:
		return "n/a"
	}
	return ConfidenceMedium
}
