// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"sort"
)

// Evaluation is the assessment of one item against what the character has equipped: for choosing a quest
// reward, but equally for any candidate (bags, vendor).
type Evaluation struct {
	Equippable    bool
	Usable        bool // false when the addon marked the item as unusable for the character (usable = false)
	LevelOk       bool // the character already has the required level
	RequiredLevel int64
	Slot          string             // the slot where the item does best ("" = cannot be equipped)
	SlotEmpty     bool               // nothing in this slot
	Replaced      []string           // names of the items the candidate would replace in Slot
	Delta         map[string]float64 // difference against what is equipped in this slot (positive = gain)
	Score         float64            // weighted difference (when weights exist), and without weights: item level difference + DPS
	Weighted      bool               // Score was computed from stat weights
	Ignored       []string           // stats without a weight, skipped in Score
	SellValue     int64              // vendor value for the whole stack (sellPrice x units)
}

// Upgrade says whether the assessment indicates an upgrade: it can be equipped, the character can use it and the score is positive.
func (e Evaluation) Upgrade() bool { return e.Equippable && e.Usable && e.Score > 0 }

// Evaluate assesses an item against the buyer's equipment. Without weights (nil or empty) it counts the change in item
// level and DPS, like the hints in wow_vendor; with weights, the sum of stat differences times the weight.
func (b Buyer) Evaluate(it Item, w Weights) Evaluation {
	e := Evaluation{
		Equippable: it.Equippable(), Usable: !it.Unusable,
		RequiredLevel: it.RequiredLevel, LevelOk: it.LevelOk(b.Level), SellValue: it.SellValue(),
	}
	if !e.Equippable {
		return e
	}
	best := math.Inf(-1)
	unweighted := map[string]bool{}
	for _, c := range GearOf(b.Equipped).Compare(it) {
		score := c.UpgradeScore()
		if len(w) > 0 {
			var skipped []string
			score, skipped = w.score(c.Delta)
			for _, stat := range skipped {
				unweighted[stat] = true
			}
		}
		if score > best {
			best = score
			e.Slot, e.Delta, e.Score, e.SlotEmpty = c.Slot, c.Delta, score, c.Empty()
			e.Replaced = e.Replaced[:0]
			for _, w := range c.Replaced {
				e.Replaced = append(e.Replaced, w.nameLabel())
			}
		}
	}
	e.Weighted = len(w) > 0
	for stat := range unweighted {
		e.Ignored = append(e.Ignored, stat)
	}
	sort.Strings(e.Ignored)
	return e
}
