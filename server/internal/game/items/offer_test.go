// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"testing"
)

func TestBuyerAdmitsAndJudge(t *testing.T) {
	buyer := Buyer{Level: 12, Money: 500}
	bow := map[string]any{"name": "Bow", "equipLoc": "INVTYPE_RANGEDRIGHT", "requiredLevel": int64(14), "quality": int64(2)}
	if buyer.Admits(bow, 100, true, OfferFilter{Usable: true}) {
		t.Error("level 14 is not usable at level 12")
	}
	if buyer.Admits(bow, 900, true, OfferFilter{Affordable: true}) || !buyer.Admits(bow, 0, false, OfferFilter{Affordable: true}) {
		t.Error("affordable: a known price above the money is out, an unknown price stays")
	}
	if buyer.Admits(map[string]any{"name": "Thread"}, 1, true, OfferFilter{Equippable: true}) {
		t.Error("a trade good is not equippable")
	}
	entry := map[string]any{}
	buyer.Judge(entry, bow, 900, true)
	if entry["canAfford"] != false || entry["levelOk"] != false || entry["qualityName"] != "Uncommon" {
		t.Errorf("judge: %v", entry)
	}
}

func TestMatchesTypeIgnoresPluralAndUpgradeScoreSortsMissingLast(t *testing.T) {
	bow := map[string]any{"type": "Weapon", "subType": "Bows"}
	for _, kind := range []string{"Bow", "bows", "Weapon"} {
		if !MatchesType(bow, kind) {
			t.Errorf("%q should match", kind)
		}
	}
	if MatchesType(bow, "Armor") {
		t.Error("a bow is not armor")
	}
	if !math.IsInf(UpgradeScore(map[string]any{}), -1) {
		t.Error("no comparison sorts last")
	}
	up := map[string]any{"vsEquipped": map[string]any{"delta": map[string]float64{"Item Level": 5, "DPS": 1.5}}}
	if UpgradeScore(up) != 6.5 {
		t.Errorf("score: %v", UpgradeScore(up))
	}
}
