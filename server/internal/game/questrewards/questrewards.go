// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package questrewards rates the choosable quest rewards: for every quest in the log and for the quest
// an NPC is currently offering (questOffer) or accepting (questTurnIn), it compares each reward
// with the equipped gear and points to the best choice with a justification. It only reads the dump; it chooses nothing in game.
package questrewards

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_quest_rewards.
type Query struct {
	Character string             `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Quest     string             `json:"quest,omitempty" jsonschema:"quest name (or fragment) or ID; empty = all quests with rewards"`
	Weights   map[string]float64 `json:"weights,omitempty" jsonschema:"optional stat weights, e.g. {\"agility\":1,\"attackPower\":0.5}; without them the server uses weights.json (if present), and without that item level and DPS"`
}

// Where the quest reward rating comes from.
const (
	fromTurnIn = "turnIn" // quest turn-in window at the NPC
	fromOffer  = "offer"  // quest offer window at the NPC
	fromLog    = "log"    // quest log
)

// suspectNote explains why the rewards from the log may be false.
const suspectNote = "Several quests in the log have exactly the same set of choosable rewards in the same order. " +
	"The client may have returned the rewards of the selected quest for all of them (the addon reads them by quest ID), so this may be false data. " +
	"I do not recommend a choice; check the rewards in game (the NPC quest window is more reliable: open it and run /mcp reload)."

// entry is a quest with rewards to rate.
type entry struct {
	source  string
	id      int64
	title   string
	npc     string
	section map[string]any // window or log entry (for meta and age)
	rewards map[string]any
	done    bool // for the log: quest completed
}

// View rates the rewards of quests from the log and NPC windows against the character's equipment.
func View(s snapshot.Snapshot, q Query, sc items.Scoring, fresh snapshot.Freshness) (map[string]any, error) {
	weights, source, weightsNote, err := items.ChooseWeights(q.Weights, sc, s)
	if err != nil {
		return nil, err
	}
	ev := evaluator{snap: s, weights: weights}
	out := map[string]any{"scoring": ev.describe(source, weightsNote)}

	entries := windowEntries(s)
	shownInWindow := map[int64]bool{}
	for _, e := range entries {
		shownInWindow[e.id] = true
	}
	logged := logEntries(s)
	suspect := suspectQuests(logged)
	for _, e := range logged {
		if !shownInWindow[e.id] || e.id == 0 {
			entries = append(entries, e)
		}
	}

	views := []any{}
	var fixedOnly []any
	for _, e := range entries {
		if !q.matches(e) {
			continue
		}
		if len(snapshot.AsList(e.rewards["choices"])) == 0 {
			if e.source == fromLog {
				fixedOnly = append(fixedOnly, e.fixedOnlyView())
				continue
			}
		}
		views = append(views, ev.view(e, suspect[e.id] && e.source == fromLog, shownInWindow, fresh))
	}
	out["quests"] = views
	out["count"] = len(views)
	if len(fixedOnly) > 0 {
		out["withoutChoices"] = fixedOnly
	}
	if len(suspect) > 0 {
		out["suspectNote"] = suspectNote
	}
	switch {
	case len(views) == 0 && len(fixedOnly) == 0 && !hasAnyRewards(s):
		out["note"] = "The dump has no quest rewards: addon 0.6.0 saves quests[].rewards and the questOffer and questTurnIn sections " +
			"(a quest offered or accepted by an NPC). Update the addon and run /mcp reload; you can also just open the NPC quest window."
	case len(views) == 0:
		out["note"] = "No quests with choosable rewards" + queryText(q) + "."
	}
	return out, nil
}

func queryText(q Query) string {
	if q.Quest == "" {
		return ""
	}
	return " matching " + strconv.Quote(q.Quest)
}

// hasAnyRewards says whether the dump has any reward data (log or NPC windows).
func hasAnyRewards(s snapshot.Snapshot) bool {
	if s.Map("questOffer")["rewards"] != nil || s.Map("questTurnIn")["rewards"] != nil {
		return true
	}
	for _, raw := range s.List("quests") {
		if snapshot.AsMap(raw)["rewards"] != nil {
			return true
		}
	}
	return false
}

// matches applies the quest filter: ID or a case-insensitive title fragment.
func (q Query) matches(e entry) bool {
	needle := strings.TrimSpace(q.Quest)
	if needle == "" {
		return true
	}
	if id, err := strconv.ParseInt(needle, 10, 64); err == nil {
		return e.id == id
	}
	return strings.Contains(strings.ToLower(e.title), strings.ToLower(needle))
}

// windowEntries returns quests from the NPC windows: turn-ins first, then offers.
func windowEntries(s snapshot.Snapshot) []entry {
	var out []entry
	for _, w := range []struct{ section, source string }{{"questTurnIn", fromTurnIn}, {"questOffer", fromOffer}} {
		sec := s.Map(w.section)
		if sec == nil {
			continue
		}
		id, _ := snapshot.AsInt(sec["id"])
		title, _ := sec["title"].(string)
		npc, _ := sec["npc"].(string)
		out = append(out, entry{source: w.source, id: id, title: title, npc: npc, section: sec, rewards: snapshot.AsMap(sec["rewards"])})
	}
	return out
}

// logEntries returns quests from the log that have rewards saved; completed ones first.
func logEntries(s snapshot.Snapshot) []entry {
	var out []entry
	for _, raw := range s.List("quests") {
		q := snapshot.AsMap(raw)
		rewards := snapshot.AsMap(q["rewards"])
		if len(rewards) == 0 {
			continue
		}
		id, _ := snapshot.AsInt(q["id"])
		title, _ := q["title"].(string)
		done, _ := q["complete"].(bool)
		out = append(out, entry{source: fromLog, id: id, title: title, rewards: rewards, done: done})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].done && !out[j].done })
	return out
}

// choiceSignature is the IDs of the choosable rewards in order; "" when the quest has no rewards with IDs.
func choiceSignature(rewards map[string]any) string {
	var ids []string
	for _, raw := range snapshot.AsList(rewards["choices"]) {
		id, ok := snapshot.AsInt(snapshot.AsMap(raw)["id"])
		if !ok {
			return ""
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	return strings.Join(ids, ",")
}

// suspectQuests returns the IDs of log quests whose non-empty choosable reward list (the same items in the same
// order) repeats on at least two quests. This is the pattern a client would give if it ignored the quest ID and returned
// the selected entry's rewards for all of them.
func suspectQuests(logged []entry) map[int64]bool {
	groups := map[string][]int64{}
	for _, e := range logged {
		if sig := choiceSignature(e.rewards); sig != "" {
			groups[sig] = append(groups[sig], e.id)
		}
	}
	out := map[int64]bool{}
	for _, ids := range groups {
		if len(ids) >= 2 {
			for _, id := range ids {
				out[id] = true
			}
		}
	}
	return out
}

// fixedRewards describes the fixed rewards: items, gold, XP and currencies.
func fixedRewards(rewards map[string]any) map[string]any {
	out := map[string]any{}
	var fixed []string
	for _, raw := range snapshot.AsList(rewards["items"]) {
		it := snapshot.AsMap(raw)
		fixed = append(fixed, fmt.Sprintf("%v x%d", it["name"], snapshot.ItemCount(it)))
	}
	if len(fixed) > 0 {
		out["items"] = fixed
	}
	if money, ok := snapshot.AsInt(rewards["money"]); ok && money > 0 {
		out["money"] = snapshot.Money(money).String()
	}
	if xp, ok := snapshot.AsInt(rewards["xp"]); ok && xp > 0 {
		out["xp"] = xp
	}
	if cur := snapshot.AsList(rewards["currencies"]); len(cur) > 0 {
		out["currencies"] = cur
	}
	return out
}

func (e entry) fixedOnlyView() map[string]any {
	out := map[string]any{"title": e.title, "id": e.id, "complete": e.done}
	snapshot.Merge(out, fixedRewards(e.rewards))
	return out
}
