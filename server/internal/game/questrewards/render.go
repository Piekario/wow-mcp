// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questrewards

import (
	"fmt"
	"strconv"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// suspectNote explains why the rewards from the log may be false.
const suspectNote = "Several quests in the log have exactly the same set of choosable rewards in the same order. " +
	"The client may have returned the rewards of the selected quest for all of them (the addon reads them by quest ID), so this may be false data. " +
	"I do not recommend a choice; check the rewards in game (the NPC quest window is more reliable: open it and run /mcp reload)."

// View rates the rewards of quests from the log and NPC windows against the character's equipment.
func View(s snapshot.Snapshot, q Query, sc items.Scoring, fresh snapshot.Freshness) (map[string]any, error) {
	report, err := analyse(s, q, sc, fresh)
	if err != nil {
		return nil, err
	}
	return report.Render(fresh), nil
}

func (r Report) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"scoring": r.Method.render()}
	views := []any{}
	for _, j := range r.Judged {
		views = append(views, j.render(fresh))
	}
	out["quests"] = views
	out["count"] = len(views)
	if len(r.WithoutChoices) > 0 {
		fixedOnly := []any{}
		for _, e := range r.WithoutChoices {
			fixedOnly = append(fixedOnly, e.fixedOnlyView())
		}
		out["withoutChoices"] = fixedOnly
	}
	if r.AnySuspect {
		out["suspectNote"] = suspectNote
	}
	switch {
	case len(views) == 0 && len(r.WithoutChoices) == 0 && !r.HasAnyRewards:
		out["note"] = "The dump has no quest rewards: addon 0.6.0 saves quests[].rewards and the questOffer and questTurnIn sections " +
			"(a quest offered or accepted by an NPC). Update the addon and run /mcp reload; you can also just open the NPC quest window."
	case len(views) == 0:
		out["note"] = "No quests with choosable rewards" + r.filterText() + "."
	}
	return out
}

func (r Report) filterText() string {
	if r.Filter == "" {
		return ""
	}
	return " matching " + strconv.Quote(r.Filter)
}

// render says how the rating is computed (the assumptions are explicit).
func (m Method) render() map[string]any {
	out := map[string]any{}
	if len(m.Weights) > 0 {
		out["method"] = "stat weights"
		out["source"] = m.Source
		out["values"] = m.Weights
		out["how"] = "score = sum of stat differences (reward minus equipped in that slot) times weight; best slot for each reward"
	} else {
		out["method"] = "item level and DPS"
		out["how"] = "without weights the server only counts the change in item level and DPS relative to the equipped item; this is a rough approximation, " +
			"pass weights (e.g. {\"agility\":1}) or a weights.json file to take stats into account"
	}
	if m.Note != "" {
		out["weightsNote"] = m.Note
	}
	return out
}

func (r Rewards) render() map[string]any {
	out := map[string]any{}
	var fixed []string
	for _, it := range r.Fixed {
		fixed = append(fixed, fmt.Sprintf("%v x%d", it.Name, it.Count))
	}
	if len(fixed) > 0 {
		out["items"] = fixed
	}
	if r.Money > 0 {
		out["money"] = snapshot.Money(r.Money).String()
	}
	if r.XP > 0 {
		out["xp"] = r.XP
	}
	if len(r.Currencies) > 0 {
		out["currencies"] = r.Currencies
	}
	return out
}

func (e Quest) fixedOnlyView() map[string]any {
	out := map[string]any{"title": e.Title, "id": e.ID, "complete": e.Done}
	snapshot.Merge(out, e.Rewards.render())
	return out
}

func (j Judgement) render(fresh snapshot.Freshness) map[string]any {
	e := j.Quest
	out := map[string]any{"source": e.Source, "title": e.Title}
	if e.ID != 0 {
		out["id"] = e.ID
	}
	switch e.Source {
	case fromTurnIn:
		out["where"] = "quest turn-in at the NPC (questTurnIn)"
	case fromOffer:
		out["where"] = "quest offered by the NPC (questOffer)"
	default:
		out["where"] = "quest log"
		out["complete"] = e.Done
		if j.AlsoInWindow {
			out["alsoInWindow"] = true
		}
	}
	if e.NPC != "" {
		out["npc"] = e.NPC
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(e.Captured))
	if e.Rewards.HasFixed() {
		out["fixed"] = e.Rewards.render()
	}
	switch j.Verdict {
	case Suspect:
		out["suspect"] = true
		out["note"] = "The client may have returned the rewards of another quest (identical reward list on several quest log entries); verify in game. I am not recommending a choice."
		out["choices"] = e.Rewards.choiceNames()
	case Stale:
		out["note"] = "The NPC window was captured long ago; the rewards may have changed, so I am not recommending a choice. Open the quest window in game and run /mcp reload."
		out["choices"] = e.Rewards.choiceNames()
	case Rated:
		j.renderRated(out)
	}
	return out
}

func (r Rewards) choiceNames() []string {
	names := make([]string, len(r.Choices))
	for i, c := range r.Choices {
		names[i] = c.label()
	}
	return names
}

func (j Judgement) renderRated(out map[string]any) {
	views := make([]any, len(j.Choices))
	for i, c := range j.Choices {
		views[i] = c.render()
	}
	out["choices"] = views
	if j.Best == nil {
		out["note"] = "I do not have the data to recommend a choice (no upgrades and no sell prices)."
		return
	}
	best := map[string]any{"name": j.Best.label(), "reason": j.Best.reason()}
	if j.ForNow != nil {
		best["forNow"] = j.ForNow.render()
	}
	out["best"] = best
}

func (c RatedChoice) render() map[string]any {
	e := c.Eval
	out := map[string]any{
		"name": c.Name, "count": c.Count,
		"equippable": e.Equippable, "usable": e.Usable, "levelOk": e.LevelOk,
	}
	if c.ID != nil {
		out["id"] = c.ID
	}
	if e.RequiredLevel > 0 {
		out["requiredLevel"] = e.RequiredLevel
	}
	if e.SellValue > 0 {
		out["vendorValue"] = snapshot.Money(e.SellValue).String()
	}
	if e.Equippable {
		out["slot"] = e.Slot
		if e.SlotEmpty {
			out["slotEmpty"] = true
		}
		out["delta"] = e.Delta
		out["score"] = snapshot.Round1(e.Score)
		if len(e.Ignored) > 0 {
			out["ignoredStats"] = e.Ignored
		}
	}
	if !e.Usable {
		out["note"] = "the addon marked this reward as unusable by the character (usable = false): vendor value"
	}
	return out
}

func (p Pick) reason() string {
	e := p.Eval
	if !p.Upgrade {
		return "no reward is an upgrade over the equipped gear (or they cannot be used); highest vendor value: " +
			snapshot.Money(e.SellValue).String()
	}
	reason := fmt.Sprintf("biggest gain in slot %s: score %+g (%s)", e.Slot, snapshot.Round1(e.Score), basis(e))
	if e.SlotEmpty {
		reason += "; the slot is currently empty"
	}
	if !e.LevelOk {
		reason += fmt.Sprintf("; requires level %d, which the character does not have yet", e.RequiredLevel)
	}
	return reason
}

func basis(e items.Evaluation) string {
	if e.Weighted {
		return "stat weights"
	}
	return "item level + DPS"
}

func (a Alternative) render() string {
	if a.Eval.Upgrade() {
		return a.label() + " (an upgrade that can be used right away)"
	}
	return a.label() + " (usable right away; otherwise vendor value)"
}
