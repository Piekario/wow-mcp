// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package main

import (
	"fmt"
	"reflect"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/bagplan"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/reagents"
	"wowmcp/internal/snapshot"
)

// The deDE fixture is the Forever mock with the client text in German (types, subtypes, tooltips, stat labels,
// class, profession and some item names) and the same IDs. Whatever the server concludes from item classes,
// binding, stats and professions must be the same on both.

// itemsByID collects the items of bags, bank and equipment by item ID (the first instance of each).
func itemsByID(s snapshot.Snapshot) map[int64]map[string]any {
	out := map[int64]map[string]any{}
	add := func(list []any) {
		for _, raw := range list {
			item := snapshot.AsMap(raw)
			if id, ok := snapshot.AsInt(item["id"]); ok && out[id] == nil {
				out[id] = item
			}
		}
	}
	for _, bag := range s.Bags() {
		add(snapshot.AsList(snapshot.AsMap(bag)["items"]))
	}
	for _, c := range s.BankContainers() {
		add(snapshot.AsList(snapshot.AsMap(c)["items"]))
	}
	add(s.Equipment())
	return out
}

func TestDedeItemClassificationMatchesEnglish(t *testing.T) {
	en, de := itemsByID(fixtures.Forever(t)), itemsByID(fixtures.Dede(t))
	if len(en) == 0 || len(en) != len(de) {
		t.Fatalf("items: %d English, %d German", len(en), len(de))
	}
	judge := func(item map[string]any) string {
		return fmt.Sprint(itemkind.IsAmmo(item), itemkind.AmmoKind(item), itemkind.ConsumableKind(item),
			itemkind.TradeGoodsKind(item), itemkind.IsQuestItem(item), itemkind.StartsQuest(item), itemkind.IsBound(item),
			itemkind.Tradable(item), itemkind.IsGearClass(item), itemkind.RangedAmmo(item))
	}
	for id, item := range en {
		if de[id] == nil {
			t.Errorf("item %d missing in deDE", id)
			continue
		}
		if a, b := judge(item), judge(de[id]); a != b {
			t.Errorf("item %d (%v): English %s, German %s", id, item["name"], a, b)
		}
	}
	if !itemkind.IsBound(de[6948]) || itemkind.IsBound(de[2589]) {
		t.Error("binding from the container: Hearthstone bound, Linen Cloth not")
	}
}

// actionsByID is the bag plan action of every item, keyed by item ID (names differ between the clients).
func actionsByID(t *testing.T, s snapshot.Snapshot) map[int64]string {
	t.Helper()
	ids := map[string]int64{}
	for id, item := range itemsByID(s) {
		ids[fmt.Sprint(item["name"])] = id
	}
	out := map[int64]string{}
	for _, raw := range bagplan.View(s, fixtures.At(0))["items"].([]any) {
		e := raw.(map[string]any)
		out[ids[fmt.Sprint(e["name"])]] = fmt.Sprint(e["action"])
	}
	return out
}

func TestDedeBagPlanMatchesEnglish(t *testing.T) {
	en, de := actionsByID(t, fixtures.Forever(t)), actionsByID(t, fixtures.Dede(t))
	if !reflect.DeepEqual(en, de) {
		t.Errorf("bag plan:\nEnglish %v\nGerman  %v", en, de)
	}
	if de[7005] != bagplan.Keep {
		t.Errorf("Skinning Knife (Kürschnermesser) is a profession tool on a German client too: %q", de[7005])
	}
}

func TestDedeReagentCategoriesMatchEnglish(t *testing.T) {
	// The mock bags hold only Trade Goods and a weapon, so the same consumables are added to both dumps: English text
	// only (an old dump) and German text with IDs (addon 0.9.0).
	add := func(s snapshot.Snapshot, list ...map[string]any) snapshot.Snapshot {
		bag := snapshot.AsMap(s.Bags()[0])
		for i, item := range list {
			item["id"], item["count"], item["name"] = int64(90000+i), int64(5), fmt.Sprintf("Item %d", i)
			bag["items"] = append(snapshot.AsList(bag["items"]), item)
		}
		return s
	}
	en := add(fixtures.Forever(t),
		map[string]any{"type": "Consumable", "subType": "Food & Drink"},
		map[string]any{"type": "Consumable", "subType": "Bandage"},
		map[string]any{"type": "Consumable", "subType": "Potion"},
		map[string]any{"type": "Projectile", "subType": "Arrow"},
		map[string]any{"type": "Reagent", "subType": "Reagent"})
	de := add(fixtures.Dede(t),
		map[string]any{"type": "Verbrauchbar", "subType": "Essen & Trinken", "classID": int64(0), "subclassID": int64(5)},
		map[string]any{"type": "Verbrauchbar", "subType": "Verband", "classID": int64(0), "subclassID": int64(7)},
		map[string]any{"type": "Verbrauchbar", "subType": "Trank", "classID": int64(0), "subclassID": int64(1)},
		map[string]any{"type": "Projektil", "subType": "Pfeil", "classID": int64(6), "subclassID": int64(2)},
		map[string]any{"type": "Reagenz", "subType": "Reagenz", "classID": int64(5), "subclassID": int64(0)})
	counts := func(s snapshot.Snapshot) map[string]int {
		out := map[string]int{}
		for _, raw := range snapshot.AsList(reagents.View("Piekasz", s, progress.Tracker{})["categories"]) {
			c := snapshot.AsMap(raw)
			out[fmt.Sprint(c["category"])] = len(snapshot.AsList(c["items"]))
		}
		return out
	}
	a, b := counts(en), counts(de)
	if !reflect.DeepEqual(a, b) || len(a) != 5 {
		t.Errorf("reagent categories: English %v, German %v", a, b)
	}
}

func TestDedeStatComparisonMatchesEnglish(t *testing.T) {
	compare := func(s snapshot.Snapshot) (any, any) {
		out, err := items.CompareView(s, items.CompareQuery{Item: "5586", Source: "vendor"})
		if err != nil {
			t.Fatal(err)
		}
		var deltas []any
		for _, raw := range out["comparisons"].([]any) {
			deltas = append(deltas, raw.(map[string]any)["delta"])
		}
		return deltas, out["weaponSkill"] != nil
	}
	enDelta, enSkill := compare(fixtures.Forever(t))
	deDelta, deSkill := compare(fixtures.Dede(t))
	if !reflect.DeepEqual(enDelta, deDelta) || enSkill != deSkill || deSkill != true {
		t.Errorf("comparison: English %v (skill %v), German %v (skill %v)", enDelta, enSkill, deDelta, deSkill)
	}
}

func TestDedeProfessionsMatchEnglish(t *testing.T) {
	en, de := fixtures.Forever(t), fixtures.Dede(t)
	if a, b := len(professions.Summary(en)), len(professions.Summary(de)); a != b || a == 0 {
		t.Errorf("professions: English %d, German %d", a, b)
	}
	for _, skill := range []int64{professions.SkillSkinning, professions.SkillMining, professions.SkillCooking} {
		if !professions.HasSkill(en, skill) || !professions.HasSkill(de, skill) {
			t.Errorf("skill %d: English %v, German %v", skill, professions.HasSkill(en, skill), professions.HasSkill(de, skill))
		}
	}
	if de.Locale() != "deDE" || de.EnglishClient() || !en.EnglishClient() {
		t.Error("locale")
	}
}
