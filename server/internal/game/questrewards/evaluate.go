// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questrewards

import (
	"fmt"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

func (c Choice) label() string { return fmt.Sprint(c.Name) }

// judge rates the choices of one quest against the equipped gear.
func judge(s snapshot.Snapshot, w items.Weights, e Quest, suspect bool, inWindow map[int64]bool, fresh snapshot.Freshness) Judgement {
	j := Judgement{Quest: e, AlsoInWindow: e.Source == fromLog && inWindow[e.ID]}
	switch {
	case len(e.Rewards.Choices) == 0:
		j.Verdict = NoChoices
	case suspect:
		j.Verdict = Suspect
	case e.Captured.Known && fresh.IsStale(e.Captured.At):
		j.Verdict = Stale
	default:
		j.Verdict = Rated
		j.Choices = make([]RatedChoice, len(e.Rewards.Choices))
		for i, c := range e.Rewards.Choices {
			// items.Evaluate still takes the dump item: the typed items.Item cannot be built outside items.
			j.Choices[i] = RatedChoice{c, items.Evaluate(s, snapshot.AsMap(c.Raw), w)}
		}
		j.Best = pick(j.Choices)
		if j.Best != nil {
			j.ForNow = nowAlternative(j.Choices, j.Best.RatedChoice)
		}
	}
	return j
}

// pick chooses the best reward. Upgrades (equippable, usable by the character, positive score) win,
// first those for a level the character has; a tie is broken by sell price. When there are no upgrades, the highest
// vendor value wins. nil when there is nothing to base a choice on.
func pick(list []RatedChoice) *Pick {
	var upgrade *RatedChoice
	for i := range list {
		c := &list[i]
		if c.Eval.Upgrade() && (upgrade == nil || betterUpgrade(*c, *upgrade)) {
			upgrade = c
		}
	}
	if upgrade != nil {
		return &Pick{*upgrade, true}
	}
	var sell *RatedChoice
	for i := range list {
		if c := &list[i]; c.Eval.SellValue > 0 && (sell == nil || c.Eval.SellValue > sell.Eval.SellValue) {
			sell = c
		}
	}
	if sell != nil {
		return &Pick{*sell, false}
	}
	return nil
}

// betterUpgrade: the better one is for a level the character has; then the higher score; then the higher sell price.
func betterUpgrade(a, b RatedChoice) bool {
	if a.Eval.LevelOk != b.Eval.LevelOk {
		return a.Eval.LevelOk
	}
	if a.Eval.Score != b.Eval.Score {
		return a.Eval.Score > b.Eval.Score
	}
	return a.Eval.SellValue > b.Eval.SellValue
}

// nowAlternative suggests what to take when the best reward requires a higher level: an upgrade for the current level
// or, when there is none, the highest vendor value.
func nowAlternative(list []RatedChoice, best RatedChoice) *Alternative {
	if best.Eval.LevelOk {
		return nil
	}
	var alt *RatedChoice
	for i := range list {
		c := &list[i]
		if c.label() == best.label() || !c.Eval.LevelOk {
			continue
		}
		if alt == nil || c.Eval.Score > alt.Eval.Score || (c.Eval.Score == alt.Eval.Score && c.Eval.SellValue > alt.Eval.SellValue) {
			alt = c
		}
	}
	if alt == nil {
		return nil
	}
	return &Alternative{*alt}
}
