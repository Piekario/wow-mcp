// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questrewards

import (
	"fmt"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// evaluator rates rewards against the dump's equipment, with or without stat weights.
type evaluator struct {
	snap    snapshot.Snapshot
	weights items.Weights
}

// describe says how the rating is computed (the assumptions are explicit).
func (ev evaluator) describe(source, weightsNote string) map[string]any {
	out := map[string]any{}
	if len(ev.weights) > 0 {
		out["method"] = "stat weights"
		out["source"] = source
		out["values"] = ev.weights
		out["how"] = "score = sum of stat differences (reward minus equipped in that slot) times weight; best slot for each reward"
	} else {
		out["method"] = "item level and DPS"
		out["how"] = "without weights the server only counts the change in item level and DPS relative to the equipped item; this is a rough approximation, " +
			"pass weights (e.g. {\"agility\":1}) or a weights.json file to take stats into account"
	}
	if weightsNote != "" {
		out["weightsNote"] = weightsNote
	}
	return out
}

// scored is a choosable reward together with its rating.
type scored struct {
	item map[string]any
	eval items.Evaluation
}

func (sc scored) name() string { return fmt.Sprint(sc.item["name"]) }

// view builds the response for one quest.
func (ev evaluator) view(e entry, suspect bool, inWindow map[int64]bool, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"source": e.source, "title": e.title}
	if e.id != 0 {
		out["id"] = e.id
	}
	switch e.source {
	case fromTurnIn:
		out["where"] = "quest turn-in at the NPC (questTurnIn)"
	case fromOffer:
		out["where"] = "quest offered by the NPC (questOffer)"
	default:
		out["where"] = "quest log"
		out["complete"] = e.done
		if inWindow[e.id] {
			out["alsoInWindow"] = true
		}
	}
	if e.npc != "" {
		out["npc"] = e.npc
	}
	stale := false
	if e.section != nil {
		snapshot.Merge(out, fresh.CapturedMeta(e.section))
		if at, ok := snapshot.AsInt(e.section["capturedAt"]); ok {
			stale = fresh.IsStale(at)
		}
	}
	if fixed := fixedRewards(e.rewards); len(fixed) > 0 {
		out["fixed"] = fixed
	}

	choices := snapshot.AsList(e.rewards["choices"])
	if len(choices) == 0 {
		return out
	}
	switch {
	case suspect:
		out["suspect"] = true
		out["note"] = "The client may have returned the rewards of another quest (identical reward list on several quest log entries); verify in game. I am not recommending a choice."
		out["choices"] = choiceNames(choices)
		return out
	case stale:
		out["note"] = "The NPC window was captured long ago; the rewards may have changed, so I am not recommending a choice. Open the quest window in game and run /mcp reload."
		out["choices"] = choiceNames(choices)
		return out
	}

	list := make([]scored, len(choices))
	views := make([]any, len(choices))
	for i, raw := range choices {
		it := snapshot.AsMap(raw)
		list[i] = scored{item: it, eval: items.Evaluate(ev.snap, it, ev.weights)}
		views[i] = ev.choiceView(list[i])
	}
	out["choices"] = views
	if best, reason, ok := pick(list); ok {
		out["best"] = map[string]any{"name": best.name(), "reason": reason}
		if alt, ok := nowAlternative(list, best); ok {
			out["best"].(map[string]any)["forNow"] = alt
		}
	} else {
		out["note"] = "I do not have the data to recommend a choice (no upgrades and no sell prices)."
	}
	return out
}

func choiceNames(choices []any) []string {
	names := make([]string, len(choices))
	for i, raw := range choices {
		names[i] = fmt.Sprint(snapshot.AsMap(raw)["name"])
	}
	return names
}

// choiceView describes one reward with its rating.
func (ev evaluator) choiceView(sc scored) map[string]any {
	e := sc.eval
	out := map[string]any{
		"name": sc.item["name"], "count": snapshot.ItemCount(sc.item),
		"equippable": e.Equippable, "usable": e.Usable, "levelOk": e.LevelOk,
	}
	if sc.item["id"] != nil {
		out["id"] = sc.item["id"]
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

// pick chooses the best reward. Upgrades (equippable, usable by the character, positive score) win,
// first those for a level the character has; a tie is broken by sell price. When there are no upgrades, the highest
// vendor value wins. ok = false when there is nothing to base a choice on.
func pick(list []scored) (best scored, reason string, ok bool) {
	var upgrade *scored
	for i := range list {
		c := &list[i]
		if !c.eval.Upgrade() {
			continue
		}
		if upgrade == nil || betterUpgrade(*c, *upgrade) {
			upgrade = c
		}
	}
	if upgrade != nil {
		e := upgrade.eval
		reason = fmt.Sprintf("biggest gain in slot %s: score %+g (%s)", e.Slot, snapshot.Round1(e.Score), basis(e))
		if e.SlotEmpty {
			reason += "; the slot is currently empty"
		}
		if !e.LevelOk {
			reason += fmt.Sprintf("; requires level %d, which the character does not have yet", e.RequiredLevel)
		}
		return *upgrade, reason, true
	}
	var sell *scored
	for i := range list {
		if c := &list[i]; c.eval.SellValue > 0 && (sell == nil || c.eval.SellValue > sell.eval.SellValue) {
			sell = c
		}
	}
	if sell != nil {
		return *sell, "no reward is an upgrade over the equipped gear (or they cannot be used); highest vendor value: " +
			snapshot.Money(sell.eval.SellValue).String(), true
	}
	return scored{}, "", false
}

// betterUpgrade: the better one is for a level the character has; then the higher score; then the higher sell price.
func betterUpgrade(a, b scored) bool {
	if a.eval.LevelOk != b.eval.LevelOk {
		return a.eval.LevelOk
	}
	if a.eval.Score != b.eval.Score {
		return a.eval.Score > b.eval.Score
	}
	return a.eval.SellValue > b.eval.SellValue
}

func basis(e items.Evaluation) string {
	if e.Weighted {
		return "stat weights"
	}
	return "item level + DPS"
}

// nowAlternative suggests what to take when the best reward requires a higher level: an upgrade for the current level
// or, when there is none, the highest vendor value.
func nowAlternative(list []scored, best scored) (string, bool) {
	if best.eval.LevelOk {
		return "", false
	}
	var alt *scored
	for i := range list {
		c := &list[i]
		if c.name() == best.name() || !c.eval.LevelOk {
			continue
		}
		if alt == nil || c.eval.Score > alt.eval.Score || (c.eval.Score == alt.eval.Score && c.eval.SellValue > alt.eval.SellValue) {
			alt = c
		}
	}
	if alt == nil {
		return "", false
	}
	if alt.eval.Upgrade() {
		return alt.name() + " (an upgrade that can be used right away)", true
	}
	return alt.name() + " (usable right away; otherwise vendor value)", true
}
