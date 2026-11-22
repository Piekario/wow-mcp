// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"sort"

	"wowmcp/internal/snapshot"
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
	Delta         map[string]float64 // difference against what is equipped in this slot (positive = gain)
	Score         float64            // weighted difference (when weights exist), and without weights: item level difference + DPS
	Weighted      bool               // Score was computed from stat weights
	Ignored       []string           // stats without a weight, skipped in Score
	SellValue     int64              // vendor value for the whole stack (sellPrice x units)
}

// Upgrade says whether the assessment indicates an upgrade: it can be equipped, the character can use it and the score is positive.
func (e Evaluation) Upgrade() bool { return e.Equippable && e.Usable && e.Score > 0 }

// Evaluate assesses an item against the character's equipment. Without weights (nil or empty) it counts the change in item
// level and DPS, like the hints in wow_vendor; with weights, the sum of stat differences times the weight.
func Evaluate(s snapshot.Snapshot, item map[string]any, w Weights) Evaluation {
	level, _ := s.LevelAndMoney()
	req, _ := snapshot.AsInt(item["requiredLevel"])
	sell, _ := snapshot.AsInt(item["sellPrice"])
	e := Evaluation{
		Equippable: Equippable(item), Usable: item["usable"] != false,
		RequiredLevel: req, LevelOk: req <= level, SellValue: sell * snapshot.ItemCount(item),
	}
	if !e.Equippable {
		return e
	}
	best := math.Inf(-1)
	unweighted := map[string]bool{}
	for _, raw := range Compare(item, s.Equipment()) {
		c := snapshot.AsMap(raw)
		delta, _ := c["delta"].(map[string]float64)
		score := delta["Item Level"] + delta["DPS"]
		if len(w) > 0 {
			var skipped []string
			score, skipped = w.score(delta)
			for _, stat := range skipped {
				unweighted[stat] = true
			}
		}
		if score > best {
			best = score
			e.Slot, e.Delta, e.Score = c["slot"].(string), delta, score
			e.SlotEmpty = c["equipped"] == snapshot.Empty
		}
	}
	e.Weighted = len(w) > 0
	for stat := range unweighted {
		e.Ignored = append(e.Ignored, stat)
	}
	sort.Strings(e.Ignored)
	return e
}
