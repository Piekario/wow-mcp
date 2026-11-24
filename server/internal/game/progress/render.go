// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"math"
	"time"

	"wowmcp/internal/game/character"
	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/periods"
	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// maxLootListed is how many notable loot entries wow_changes shows.
const maxLootListed = 10

// NoteSkipped adds the number of skipped empty-client dumps to the response (when there are any).
func (t Tracker) NoteSkipped(out map[string]any, name string, current snapshot.Snapshot) {
	if n := t.SkippedUnloaded(name, current); n > 0 {
		out["skippedUnloadedSnapshots"] = n
		out["skippedUnloadedNote"] = snapshot.UnloadedNote
	}
}

// Changes compares the current dump with an earlier one from history (wow_changes).
func (t Tracker) Changes(character string, current snapshot.Snapshot, q ChangesQuery) (map[string]any, error) {
	r, err := t.changes(character, current, q)
	if err != nil {
		return nil, err
	}
	if r.Unloaded {
		out := map[string]any{"note": unloadedCurrentNote}
		t.NoteSkipped(out, character, current)
		return out, nil
	}
	available := []string{}
	for _, at := range r.History {
		available = append(available, snapshot.FormatTime(at))
	}
	if r.Comparison == nil {
		return map[string]any{
			"history": available,
			"note":    "This is the first remembered dump of this character, so there is nothing to compare with. Changes will be visible after the next /mcp reload in game.",
		}, nil
	}
	out := r.Comparison.Render()
	out["history"] = available
	t.NoteSkipped(out, character, current)
	return out, nil
}

// SessionChanges are the changes since the start of the current session (at minimum since the previous dump).
func (t Tracker) SessionChanges(character string, current snapshot.Snapshot) (map[string]any, error) {
	q := ChangesQuery{}
	if points, err := t.points(character, current); err == nil && len(points) > 1 {
		start := sessionStart(points, 0)
		hours := float64(points[len(points)-1].at-points[start].at)/3600 + 0.01
		if start < len(points)-1 {
			q.SinceHours = hours
		}
	}
	return t.Changes(character, current, q)
}

// Diff describes what changed between two dumps of the same character; empty categories are omitted.
func Diff(before, after snapshot.Snapshot) map[string]any {
	return Compare(before, after).Render()
}

func (c Comparison) Render() map[string]any {
	out := map[string]any{}
	if c.HasFrom {
		out["from"] = snapshot.FormatTime(c.From)
		if c.HasTo {
			out["to"] = snapshot.FormatTime(c.To)
			out["elapsed"] = snapshot.FormatSeconds(c.To - c.From)
		}
	}
	if c.Money != nil {
		out["money"] = map[string]any{"before": snapshot.Money(c.Money.Before).String(), "after": snapshot.Money(c.Money.After).String(), "change": snapshot.Money(c.Money.Delta()).Signed()}
	}
	if c.XP != nil {
		out["experience"] = c.XP.render()
	}
	if c.Zone != nil {
		out["zone"] = map[string]any{"before": c.Zone.Before, "after": c.Zone.After}
	}
	if len(c.Gained)+len(c.Lost) > 0 {
		items := map[string]any{}
		if len(c.Gained) > 0 {
			items["gained"] = renderItems(c.Gained)
		}
		if len(c.Lost) > 0 {
			items["lost"] = renderItems(c.Lost)
		}
		out["bags"] = items
	}
	if len(c.Equipment) > 0 {
		out["equipment"] = renderGear(c.Equipment)
	}
	if !c.Quests.Empty() {
		out["quests"] = c.Quests.render()
	}
	c.renderLists(out)
	c.renderGains(out)
	if !c.HasChanges() {
		out["note"] = "No changes between these dumps."
	}
	return out
}

func (c Comparison) renderLists(out map[string]any) {
	if len(c.Reputation) > 0 {
		out["reputation"] = renderReputation(c.Reputation)
	}
	if len(c.NewSpells) > 0 {
		list := make([]any, len(c.NewSpells))
		for i, s := range c.NewSpells {
			entry := map[string]any{"name": s.Name}
			if s.HasRank {
				entry["rank"] = s.Rank
			}
			list[i] = entry
		}
		out["newSpells"] = list
	}
	if len(c.Professions) > 0 {
		list := make([]any, len(c.Professions))
		for i, p := range c.Professions {
			if p.New {
				list[i] = map[string]any{"name": p.Name, "after": p.After, "max": p.Max, "note": "new"}
			} else {
				list[i] = map[string]any{"name": p.Name, "before": p.Before, "after": p.After, "max": p.Max}
			}
		}
		out["professions"] = list
	}
	if len(c.Currencies) > 0 {
		list := make([]any, len(c.Currencies))
		for i, cur := range c.Currencies {
			list[i] = map[string]any{"name": cur.Name, "before": cur.Before, "after": cur.After, "change": cur.Delta()}
		}
		out["currencies"] = list
	}
}

func (c Comparison) renderGains(out map[string]any) {
	if c.PlayedBetween > 0 {
		out["playedBetween"] = snapshot.FormatSeconds(c.PlayedBetween)
	}
	if c.Deaths != nil {
		out["deaths"] = map[string]any{"before": c.Deaths.Before, "after": c.Deaths.After, "new": c.Deaths.Delta()}
	}
	c.renderHonor(out)
	if c.Loot != nil {
		l := loot.Describe(c.Loot.Items, maxLootListed)
		c.coverage(l, c.Loot.Merge, "loot from earlier saves between these dumps is not included: the loot section of the newer dump counts from that moment")
		out["loot"] = l
	}
	if c.XPSources != nil {
		x := xpgains.Describe(c.XPSources.Gains)
		c.coverage(x, c.XPSources.Merge, "XP from earlier saves between these dumps is not included: the xpGains section of the newer dump counts from that moment")
		out["xpSources"] = x
	}
	if c.TalentPoints != nil {
		out["talentPointsSpent"] = map[string]any{"before": c.TalentPoints.Before, "after": c.TalentPoints.After}
	}
	if len(c.TalentsLearned) > 0 {
		out["talentsLearned"] = c.TalentsLearned
	}
}

func (c Comparison) renderHonor(out map[string]any) {
	if c.Honor != nil {
		out["honor"] = map[string]any{"before": c.Honor.Before, "after": c.Honor.After, "change": c.Honor.Delta()}
	}
	if c.HonorableKills != nil {
		out["honorableKills"] = map[string]any{"before": c.HonorableKills.Before, "after": c.HonorableKills.After, "new": c.HonorableKills.Delta()}
	}
	if c.HonorLevel != nil {
		out["honorLevel"] = map[string]any{"before": c.HonorLevel.Before, "after": c.HonorLevel.After,
			"note": "the honor counter restarts at a new honor level, so the honor change alone does not show what was earned"}
	}
}

// coverage adds since (the start of the earliest merged period), the number of merged carried periods and,
// when the merged periods start after the older dump, the partialNote.
func (c Comparison) coverage(out map[string]any, m periods.Merge, partialNote string) {
	since := m.Earliest()
	if since <= 0 {
		return
	}
	out["since"] = snapshot.FormatTime(since)
	if c.HasFrom && since > c.From {
		out["partialNote"] = partialNote
	}
	m.Describe(out)
}

func (x XPChange) render() map[string]any {
	out := map[string]any{"levelBefore": x.LevelBefore, "levelAfter": x.LevelAfter, "xpBefore": x.XPBefore, "xpAfter": x.XPAfter}
	if x.GainedKnown {
		out["xpGained"] = x.Gained
	}
	if x.LevelAfter != x.LevelBefore {
		out["levelUp"] = x.LevelUp()
	}
	return out
}

func renderItems(items []ItemChange) []any {
	out := make([]any, len(items))
	for i, it := range items {
		out[i] = map[string]any{"name": it.Name, "id": it.ID, "before": it.Before, "after": it.After, "change": it.Delta()}
	}
	return out
}

func renderGear(gear []GearChange) []any {
	out := make([]any, len(gear))
	for i, g := range gear {
		out[i] = map[string]any{"slot": g.Slot, "before": snapshot.OrEmpty(g.Before), "after": snapshot.OrEmpty(g.After)}
	}
	return out
}

func (q QuestChanges) render() map[string]any {
	out := map[string]any{}
	set := func(k string, refs []QuestRef) {
		if len(refs) == 0 {
			return
		}
		list := make([]any, len(refs))
		for i, r := range refs {
			list[i] = r.render()
		}
		out[k] = list
	}
	set("new", q.New)
	set("readyToTurnIn", q.ReadyToTurnIn)
	set("turnedIn", q.TurnedIn)
	set("removedFromLog", q.RemovedFromLog)
	if len(q.Progress) > 0 {
		list := make([]any, len(q.Progress))
		for i, p := range q.Progress {
			objectives := make([]any, len(p.Objectives))
			for j, o := range p.Objectives {
				objectives[j] = map[string]any{"text": o.Text, "before": o.Before, "after": o.After, "need": o.Need}
			}
			list[i] = map[string]any{"title": p.Title, "id": p.ID, "objectives": objectives}
		}
		out["progress"] = list
	}
	if q.CompletedCountChange > 0 {
		out["completedCountChange"] = q.CompletedCountChange
	}
	return out
}

func (r QuestRef) render() map[string]any {
	out := map[string]any{"title": r.Title, "id": r.ID}
	if r.HasZone {
		out["zone"] = r.Zone
	}
	return out
}

func renderReputation(changes []ReputationChange) []any {
	out := make([]any, len(changes))
	for i, r := range changes {
		if r.New {
			out[i] = map[string]any{"name": r.Name, "standing": r.Standing, "note": "new faction"}
			continue
		}
		entry := map[string]any{"name": r.Name, "change": r.Change, "standing": r.Standing}
		if r.StandingChanged {
			entry["standingBefore"] = r.StandingBefore
		}
		out[i] = entry
	}
	return out
}

// Pace computes the rate in the current session and in the whole history (or the last sinceHours),
// level-ups and the estimated time to the next level (wow_progress).
func (t Tracker) Pace(name string, current snapshot.Snapshot, args PaceQuery) (map[string]any, error) {
	r, err := t.pace(name, current, args)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"snapshotsInHistory": r.Snapshots}
	t.NoteSkipped(out, name, current)
	if played := character.Played(current); played != nil {
		out["played"] = played
	}
	if r.Single {
		out["note"] = "There is only one dump in history so far. The rate will show after the next /mcp reload (ideally every hour of play or before logging out)."
	} else {
		out["session"] = map[string]any{"note": "The current session has one dump; run /mcp reload after a while to see the rate."}
		if r.Session != nil {
			out["session"] = r.Session.render()
		}
		if r.Overall != nil {
			overall := r.Overall.render()
			overall["window"] = r.Overall.Window
			out["overall"] = overall
		}
		if len(r.LevelUps) > 0 {
			out["levelUps"] = renderLevelUps(r.LevelUps)
		}
	}
	if r.Next != nil {
		out["nextLevel"] = r.Next.render()
	}
	return out, nil
}

func (r stretchRate) render() map[string]any {
	out := map[string]any{
		"from":        snapshot.FormatTime(r.From),
		"to":          snapshot.FormatTime(r.To),
		"snapshots":   r.Snapshots,
		"basis":       r.Basis,
		"duration":    snapshot.FormatSeconds(r.Elapsed),
		"xpGained":    r.XP,
		"moneyChange": snapshot.Money(r.Money).Signed(),
	}
	if !r.XPExact {
		out["xpNote"] = "between some dumps there were 2+ level-ups; XP from those stretches is skipped"
	}
	if r.Levels != 0 {
		out["levelsGained"] = r.Levels
	}
	if r.XPSources != nil {
		x := xpgains.Describe(r.XPSources.Gains)
		r.XPSources.Merge.Describe(x)
		if r.XPSourcesFrom > r.From {
			x["countedFrom"] = snapshot.FormatTime(r.XPSourcesFrom)
			x["partialNote"] = "XP by source is counted only from the oldest kept dump; the older long history has no breakdown"
		}
		out["xpSources"] = x
	}
	if r.HasQuests {
		out["questsTurnedIn"] = r.QuestsTurnedIn
	}
	if r.Deaths > 0 {
		out["deaths"] = r.Deaths
	}
	if r.HasPvP {
		out["honorGained"] = r.HonorGained
		out["honorableKills"] = r.HonorableKills
	}
	if r.RateKnown {
		out["xpPerHour"] = int64(math.Round(r.XPPerHour))
		out["moneyPerHour"] = snapshot.Money(r.MoneyPerHour).Signed()
	} else if r.TooShort {
		out["note"] = "stretch too short (under 5 min) to compute a rate"
	}
	return out
}

func renderLevelUps(ups []levelUp) []any {
	out := make([]any, len(ups))
	for i, u := range ups {
		e := map[string]any{"level": u.Level, "seenAt": snapshot.FormatTime(u.At)}
		if u.Played >= 0 {
			e["playedAt"] = snapshot.FormatSeconds(u.Played)
		}
		out[i] = e
	}
	return out
}

func (n nextLevel) render() map[string]any {
	out := map[string]any{"level": n.Level, "xpToGo": n.XPToGo, "percent": n.Percent}
	if n.Rested > 0 {
		out["restedXp"] = n.Rested
		out["restedNote"] = "rested XP doubles the XP from monsters until it runs out, so the real time will be shorter"
	}
	if e := eta(n.XPToGo, n.SessionRate); e != "" {
		out["etaAtSessionRate"] = e
	}
	if e := eta(n.XPToGo, n.OverallRate); e != "" {
		out["etaAtOverallRate"] = e
	}
	return out
}

// eta turns the missing XP and the rate into time.
func eta(xpToGo int64, perHour float64) string {
	if perHour <= 0 {
		return ""
	}
	d := time.Duration(float64(xpToGo) / perHour * float64(time.Hour))
	if d < time.Minute {
		return "under 1 min"
	}
	return snapshot.FormatAge(d)
}

// GoldHistory shows the character's gold from dump history: day by day (first, last, change),
// the biggest gains and expenses between consecutive dumps and the change over the whole window (wow_gold_history).
func (t Tracker) GoldHistory(name string, current snapshot.Snapshot, q GoldQuery) (map[string]any, error) {
	r, err := t.goldHistory(name, current, q)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"window": fmt.Sprintf("last %d days", r.Days), "snapshots": r.Snapshots}
	t.NoteSkipped(out, name, current)
	if r.Snapshots < 2 {
		out["note"] = "There is only one dump in this window, so there is nothing to compare gold with. History grows with every /mcp reload."
		return out, nil
	}
	first, last := r.First, r.Last
	out["from"], out["to"] = snapshot.FormatTime(first.at), snapshot.FormatTime(last.at)
	out["first"], out["last"] = snapshot.Money(first.char.Money).String(), snapshot.Money(last.char.Money).String()
	out["totalChange"] = snapshot.Money(last.char.Money - first.char.Money).Signed()
	out["totalChangeCopper"] = last.char.Money - first.char.Money
	days := make([]any, len(r.ByDay))
	for i, d := range r.ByDay {
		days[i] = map[string]any{
			"day": d.Day, "snapshots": d.Snapshots,
			"first": snapshot.Money(d.First).String(), "last": snapshot.Money(d.Last).String(), "delta": snapshot.Money(d.Last - d.First).Signed(),
		}
	}
	out["days"] = days
	if len(r.Gains) > 0 {
		out["biggestGains"] = renderMoves(r.Gains)
	}
	if len(r.Losses) > 0 {
		out["biggestLosses"] = renderMoves(r.Losses)
	}
	out["note"] = "This is the balance change between dumps, without a breakdown by source (loot, sales, auctions, repairs). A day's change is the difference from the first to the last dump of that day; " +
		"what happened between days appears only in totalChange. Days without a dump do not appear in the list."
	return out, nil
}

func renderMoves(moves []goldMove) []any {
	out := make([]any, len(moves))
	for i, m := range moves {
		out[i] = map[string]any{
			"from":    snapshot.FormatTime(m.from.at),
			"to":      snapshot.FormatTime(m.to.at),
			"elapsed": snapshot.FormatSeconds(m.to.at - m.from.at),
			"change":  snapshot.Money(m.change()).Signed(),
		}
	}
	return out
}
