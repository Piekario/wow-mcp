// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// CompareQuery is the input of wow_compare_item.
type CompareQuery struct {
	Item      string `json:"item" jsonschema:"item name (or a fragment of it) or its ID"`
	Source    string `json:"source,omitempty" jsonschema:"where to look: bags, bank, vendor; empty = everywhere"`
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	// Weights are the player's stat weights for scoring the difference; empty = weights from weights.json (for the character or class), and without them just the difference.
	Weights map[string]float64 `json:"weights,omitempty" jsonschema:"optional stat weights, e.g. {\"agility\":1,\"attackPower\":0.5}; each comparison then gets weightedDelta (sum of difference x weight). Without it the server uses weights.json if the player has one"`
}

// Scoring says where to take stat weights from when the query gives none.
type Scoring struct {
	Store WeightsStore // nil = no weights file
	Key   string       // the character key in the dump (to choose weights from the file)
}

// ItemComparison is the answer of wow_compare_item before rendering. Choices is set instead of the rest when
// several different items match the query.
type ItemComparison struct {
	Choices        []FoundItem
	Candidate      FoundItem
	LevelOk        bool
	CanAfford      bool
	Slots          []SlotComparison
	Scores         SlotScores
	WeaponSkill    *WeaponSkill
	EnglishTooltip bool
}

// CompareItem compares one item from bags, bank or a vendor with the equipped gear; when several different
// items match, it returns them to choose from. When stat weights exist (from the query or the file), each slot
// gets a weighted score.
func CompareItem(s snapshot.Snapshot, q CompareQuery, sc Scoring) (ItemComparison, error) {
	if strings.TrimSpace(q.Item) == "" {
		return ItemComparison{}, fmt.Errorf("give an item name or ID in the item parameter")
	}
	found := FindItems(s, q.Item, q.Source)
	if len(found) == 0 {
		return ItemComparison{}, fmt.Errorf("%q not found in bags, bank or at a vendor. The bank and vendor appear in the dump only after their window is opened and /mcp reload is run", q.Item)
	}
	if choices := distinctByName(found); len(choices) > 1 {
		return ItemComparison{Choices: choices}, nil
	}
	f := found[0]
	it := f.Item
	if !it.Equippable() {
		return ItemComparison{}, fmt.Errorf("%s cannot be equipped (no equipLoc in the dump), so there is nothing to compare with", it.nameLabel())
	}
	buyer := NewBuyer(s)
	r := ItemComparison{
		Candidate: f, LevelOk: it.LevelOk(buyer.Level), CanAfford: it.HasPrice && it.Price <= buyer.Money,
		Slots: GearOf(buyer.Equipped).Compare(it), EnglishTooltip: it.ReadsEnglishTooltip(),
	}
	scores, err := scoreSlots(r.Slots, s, q, sc)
	if err != nil {
		return ItemComparison{}, err
	}
	r.Scores = scores
	if it.Is(itemkind.ClassWeapon) {
		r.WeaponSkill = weaponSkillFor(readWeaponSkills(s), it)
	}
	return r, nil
}

// distinctByName keeps one found item per name; a later one wins, so its source is the one shown.
func distinctByName(found []FoundItem) []FoundItem {
	index := map[string]int{}
	var out []FoundItem
	for _, f := range found {
		name := f.Item.nameLabel()
		if i, seen := index[name]; seen {
			out[i] = f
			continue
		}
		index[name] = len(out)
		out = append(out, f)
	}
	return out
}
