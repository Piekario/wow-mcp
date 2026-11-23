// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "math"

// Gear is the equipped gear by slot.
type Gear map[string]Item

// GearOf indexes worn items by slot; a later item in the same slot wins, as in the dump order.
func GearOf(equipped []Item) Gear {
	g := Gear{}
	for _, it := range equipped {
		if it.HasSlot {
			g[it.Slot] = it
		}
	}
	return g
}

// SlotComparison is a candidate against what it would replace in one slot.
type SlotComparison struct {
	Slot     string
	Replaced []Item             // empty = the slot is empty
	Equipped map[string]float64 // the numbers of the replaced items
	Delta    map[string]float64 // candidate minus equipped, rounded to 0.1, without zeros
}

func (c SlotComparison) Empty() bool { return len(c.Replaced) == 0 }

// UpgradeScore is the item level plus DPS gained: the measure that picks the best slot without stat weights.
func (c SlotComparison) UpgradeScore() float64 { return c.Delta[StatItemLevel] + c.Delta[StatDPS] }

// LooksLikeUpgrade: an empty slot, or more item level or DPS.
func (c SlotComparison) LooksLikeUpgrade() bool {
	return c.Empty() || c.Delta[StatItemLevel] > 0 || c.Delta[StatDPS] > 0
}

// Compare compares an item with what is equipped in each slot it fits; nil when it cannot be equipped.
// A two-handed weapon is compared with the sum of mainHand + offHand, because it removes both.
func (g Gear) Compare(cand Item) []SlotComparison {
	slots := cand.Slots()
	if len(slots) == 0 {
		return nil
	}
	candNums := cand.Numbers()
	out := make([]SlotComparison, 0, len(slots))
	for _, slot := range slots {
		var replaced []Item
		if w, ok := g[slot]; ok {
			replaced = append(replaced, w)
		}
		if off, ok := g["offHand"]; ok && cand.TwoHanded() {
			replaced = append(replaced, off)
		}
		current := wornNumbers(replaced)
		out = append(out, SlotComparison{Slot: slot, Replaced: replaced, Equipped: current,
			Delta: numbersDelta(candNums, current, cand.HasTooltip)})
	}
	return out
}

// wornNumbers sums the numbers of the items being removed; speed and item level
// make sense only for a single item, so they are skipped for two.
func wornNumbers(replaced []Item) map[string]float64 {
	current := map[string]float64{}
	for _, w := range replaced {
		for k, v := range w.Numbers() {
			if k == StatSpeed || k == StatItemLevel {
				if len(replaced) == 1 {
					current[k] = v
				}
				continue
			}
			current[k] += v
		}
	}
	return current
}

func numbersDelta(cand, current map[string]float64, hasTooltip bool) map[string]float64 {
	keys := map[string]bool{}
	for k := range cand {
		keys[k] = true
	}
	for k := range current {
		keys[k] = true
	}
	delta := map[string]float64{}
	for k := range keys {
		if k == StatSpeed {
			continue // speed is neither gain nor loss; it is in both items' numbers
		}
		if _, known := cand[k]; (k == StatDPS || k == StatArmor) && !hasTooltip && !known {
			continue // without a tooltip or numbers from the addon we do not know the candidate's damage or armor
		}
		if d := math.Round((cand[k]-current[k])*10) / 10; d != 0 {
			delta[k] = d
		}
	}
	return delta
}

// bestSlot picks the comparison with the highest UpgradeScore; the first one wins a tie.
func bestSlot(cmp []SlotComparison) (SlotComparison, bool) {
	var best SlotComparison
	found := false
	bestScore := math.Inf(-1)
	for _, c := range cmp {
		if score := c.UpgradeScore(); score > bestScore {
			best, bestScore, found = c, score, true
		}
	}
	return best, found
}
