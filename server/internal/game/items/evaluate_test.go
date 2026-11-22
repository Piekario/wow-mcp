// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"testing"

	"wowmcp/internal/fixtures"
)

func TestEvaluateWithoutWeights(t *testing.T) {
	s := fixtures.Forever(t)
	dagger := map[string]any{"name": "Dagger X", "equipLoc": "INVTYPE_WEAPON", "itemLevel": int64(99), "sellPrice": int64(10), "count": int64(1)}
	e := Evaluate(s, dagger, nil)
	if !e.Equippable || !e.Usable || e.Slot == "" || e.Weighted || e.SellValue != 10 {
		t.Fatalf("assessment: %+v", e)
	}
	if e.Score <= 0 || !e.Upgrade() {
		t.Errorf("level 99 should be an upgrade: %+v", e)
	}
}

func TestEvaluateUnusableAndNotEquippable(t *testing.T) {
	s := fixtures.Forever(t)
	junk := map[string]any{"name": "Webwood Slicer", "usable": false, "sellPrice": int64(13), "count": int64(2)}
	e := Evaluate(s, junk, nil)
	if e.Equippable || e.Usable || e.Upgrade() || e.SellValue != 26 {
		t.Errorf("unusable item: %+v", e)
	}
}

func TestEvaluateWeightsAndLevel(t *testing.T) {
	s := fixtures.Forever(t)
	boots := map[string]any{"name": "Boots", "equipLoc": "INVTYPE_FEET", "requiredLevel": int64(60),
		"stats": map[string]any{"ITEM_MOD_AGILITY_SHORT": int64(10), "ITEM_MOD_STAMINA_SHORT": int64(4)}}
	e := Evaluate(s, boots, Weights{"agility": 2})
	if !e.Weighted || e.LevelOk || e.RequiredLevel != 60 {
		t.Errorf("level: %+v", e)
	}
	if e.Score < 20 {
		t.Errorf("10 agility x 2 in an empty slot or more: %v", e.Score)
	}
	if len(e.Ignored) == 0 {
		t.Errorf("stamina without a weight should be reported: %+v", e)
	}
}

func TestChooseWeightsArgumentBeatsFileAndRejectsNaN(t *testing.T) {
	s := fixtures.Forever(t)
	w, src, note, err := ChooseWeights(map[string]float64{"agility": 1}, Scoring{}, s)
	if err != nil || note != "" || src != "argument weights" || w["agility"] != 1 {
		t.Errorf("argument: %v %q %q %v", w, src, note, err)
	}
	nan := 0.0
	nan = nan / nan
	if _, _, _, err := ChooseWeights(map[string]float64{"agility": nan}, Scoring{}, s); err == nil {
		t.Error("NaN in the argument must be an error")
	}
	if w, _, _, _ := ChooseWeights(nil, Scoring{}, s); len(w) != 0 {
		t.Error("without an argument and a file there are no weights")
	}
}
