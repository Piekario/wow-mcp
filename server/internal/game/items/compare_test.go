// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"testing"

	"wowmcp/internal/game/itemkind"
)

func worn(slot string, itemLevel float64, stats map[string]float64) Item {
	return Item{Slot: slot, HasSlot: true, ItemLevel: itemLevel, HasItemLevel: true, Stats: stats, present: true}
}

func TestTwoHandedWeaponReplacesBothHands(t *testing.T) {
	gear := GearOf([]Item{
		worn("mainHand", 20, map[string]float64{"ITEM_MOD_STRENGTH_SHORT": 4}),
		worn("offHand", 18, map[string]float64{"ITEM_MOD_STAMINA_SHORT": 3}),
	})
	axe := Item{EquipLoc: "INVTYPE_2HWEAPON", ItemLevel: 25, HasItemLevel: true,
		Stats: map[string]float64{"ITEM_MOD_STRENGTH_SHORT": 10}}
	cmp := gear.Compare(axe)
	if len(cmp) != 1 || cmp[0].Slot != "mainHand" || len(cmp[0].Replaced) != 2 {
		t.Fatalf("a two-handed weapon goes to mainHand and removes both: %+v", cmp)
	}
	d := cmp[0].Delta
	if d["Strength"] != 6 || d["Stamina"] != -3 {
		t.Errorf("stats of both removed items are summed: %v", d)
	}
	if d[StatItemLevel] != 25 {
		t.Errorf("item level of two removed items is not summed, so the delta is the full item level: %v", d)
	}
}

func TestEmptySlotLooksLikeUpgradeAndBestSlotPrefersHigherScore(t *testing.T) {
	gear := GearOf([]Item{worn("finger1", 30, nil)})
	ring := Item{EquipLoc: "INVTYPE_FINGER", ItemLevel: 20, HasItemLevel: true}
	cmp := gear.Compare(ring)
	if len(cmp) != 2 || cmp[0].Empty() || !cmp[1].Empty() {
		t.Fatalf("finger1 worn, finger2 empty: %+v", cmp)
	}
	if cmp[0].LooksLikeUpgrade() || !cmp[1].LooksLikeUpgrade() {
		t.Error("a lower item level is no upgrade, an empty slot is")
	}
	if best, ok := bestSlot(cmp); !ok || best.Slot != "finger2" {
		t.Errorf("best slot: %+v", best)
	}
	if (Gear{}).Compare(Item{Item: itemkind.Item{Name: "Linen Cloth"}}) != nil {
		t.Error("an item without a slot has no comparison")
	}
}

func TestBuyerAssessAndUpgradeSlot(t *testing.T) {
	buyer := Buyer{Level: 10, Money: 100, Equipped: []Item{worn("feet", 8, nil)}}
	boots := Item{EquipLoc: "INVTYPE_FEET", ItemLevel: 12, HasItemLevel: true, RequiredLevel: 11, Quality: 2, HasQuality: true}
	o := buyer.Assess(boots, 150, true)
	if o.CanAfford || o.LevelOk || o.Quality != "Uncommon" || o.VsEquipped == nil || o.UpgradeScore() != 4 {
		t.Errorf("offer: %+v", o)
	}
	if _, ok := buyer.UpgradeSlot(boots); ok {
		t.Error("an item above the character's level is not suggested")
	}
	boots.RequiredLevel = 5
	if slot, ok := buyer.UpgradeSlot(boots); !ok || slot != "feet" {
		t.Errorf("upgrade slot: %q %v", slot, ok)
	}
	if !math.IsInf(buyer.Assess(Item{}, 0, false).UpgradeScore(), -1) {
		t.Error("an offer that cannot be equipped sorts last")
	}
}

func TestWeaponSkillByIDBeforeEnglishName(t *testing.T) {
	lines := []SkillLine{
		{ID: 173, HasID: true, Name: "Dolche", NameText: "Dolche", Group: "Waffe", Rank: int64(40)},
		{NameText: "Daggers", Group: "Weapon Skills", Name: "Daggers", Rank: int64(5)},
	}
	german := Item{Item: itemkind.Item{Class: itemkind.ClassWeapon, HasClass: true, Subclass: 15, HasSubclass: true, SubType: "Dolche"}}
	if ws := weaponSkillFor(lines, german); ws == nil || ws.Name != "Dolche" || ws.Rank != int64(40) {
		t.Errorf("skill by ID: %+v", ws)
	}
	old := Item{Item: itemkind.Item{SubType: "Dagger"}}
	if ws := weaponSkillFor(lines, old); ws == nil || ws.Name != "Daggers" {
		t.Errorf("an old dump falls back to the English subtype: %+v", ws)
	}
}

func TestScoreSlotsPicksBestWeightedSlot(t *testing.T) {
	cmp := []SlotComparison{
		{Slot: "mainHand", Delta: map[string]float64{"Agility": 2, "Stamina": 5}},
		{Slot: "offHand", Delta: map[string]float64{"Agility": 4}},
	}
	r, err := scoreSlots(cmp, nil, CompareQuery{Weights: map[string]float64{"agility": 1.5}}, Scoring{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Weighted() || r.BestSlot != "offHand" || r.Best != 6 || len(r.Scores) != 2 || r.Scores[0] != 3 {
		t.Errorf("scores: %+v", r)
	}
	if len(r.Ignored) != 1 || r.Ignored[0] != "Stamina" {
		t.Errorf("ignored: %v", r.Ignored)
	}
}
