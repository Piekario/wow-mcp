// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestBankView(t *testing.T) {
	fresh := fixtures.RealTime()
	out := Bank(fixtures.Forever(t), BankQuery{Sellable: true, Type: "trade goods"}, fresh)
	if out["count"] != 1 || out["totalVendorValue"] != "2s 60c" || out["freeSlots"] != int64(23) {
		t.Errorf("bank: %v", out)
	}
	if out := Bank(fixtures.Forever(t), BankQuery{Query: "nothing like this"}, fresh); out["count"] != 0 {
		t.Errorf("name filter: %v", out)
	}
	if out := Bank(snapshot.Snapshot{}, BankQuery{}, fresh); out["note"] == nil {
		t.Error("no note when there is no bank")
	}
}

func TestVendorView(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := fixtures.RealTime()
	all := Vendor(s, VendorQuery{}, fresh)
	if all["count"] != 2 {
		t.Fatalf("offer: %v", all)
	}
	// The character has 52s 12c: the bow at 72s 95c is too expensive.
	cheap := Vendor(s, VendorQuery{Affordable: true}, fresh)
	if cheap["count"] != 1 {
		t.Errorf("affordable: %v", cheap)
	}
	gear := Vendor(s, VendorQuery{Equippable: true}, fresh)
	items := gear["items"].([]any)
	if len(items) != 1 || snapshot.AsMap(items[0])["vsEquipped"] == nil {
		t.Errorf("equippable/vsEquipped: %v", gear)
	}
}

func TestCompareItem(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := CompareView(s, CompareQuery{Item: "Skinning Knife"})
	if err != nil {
		t.Fatal(err)
	}
	cmps := out["comparisons"].([]any)
	if len(cmps) != 2 {
		t.Fatalf("one-handed weapon: expected mainHand and offHand, got %v", cmps)
	}
	main := snapshot.AsMap(cmps[0])
	delta := main["delta"].(map[string]float64)
	// Skinning Knife 2-5 / 1.6 = 2.2 DPS; Curvewood Dagger 9-18 / 1.6 = 8.4 DPS.
	if main["slot"] != "mainHand" || delta["DPS"] != -6.2 || delta["Agility"] != -3 || delta["Stamina"] != -2 {
		t.Errorf("comparison: %v", main)
	}
	if ws := snapshot.AsMap(out["weaponSkill"]); ws["name"] != "Daggers" {
		t.Errorf("weapon skill: %v", out["weaponSkill"])
	}

	// By ID and from the vendor source.
	out, err = CompareView(s, CompareQuery{Item: "5586", Source: "vendor"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AsMap(out["item"])["price"] != "72s 95c" {
		t.Errorf("price: %v", out["item"])
	}

	// "Cloth" matches only Linen Cloth (bags and bank): a single item, so no list to choose from,
	// just an error that it cannot be equipped.
	if out, err := CompareView(s, CompareQuery{Item: "Cloth"}); err == nil {
		t.Errorf("expected an error, got %v", out)
	}
	if out, _ := CompareView(s, CompareQuery{Item: "e"}); out == nil || out["ambiguous"] != true {
		t.Errorf("fragment matching many items: %v", out)
	}
	if _, err := CompareView(s, CompareQuery{Item: "Linen Cloth"}); err == nil {
		t.Error("cloth cannot be equipped, expected an error")
	}
	if _, err := CompareView(s, CompareQuery{Item: "Thunderfury"}); err == nil {
		t.Error("unknown item, expected an error")
	}
}

func TestStatName(t *testing.T) {
	cases := map[string]string{"ITEM_MOD_STAMINA_SHORT": "Stamina", "ITEM_MOD_CRIT_RATING_SHORT": "Crit Rating", "Agility": "Agility"}
	for in, want := range cases {
		if got := statName(in); got != want {
			t.Errorf("statName(%q) = %q", in, got)
		}
	}
}

func TestBagExtras(t *testing.T) {
	b := bagExtras(fixtures.Forever(t))
	fixtures.WantAll(t, "bags", fixtures.JSON(t, b), `"ammo":{"count":812,"name":"Sharp Arrow"}`)
}

// A second character (alt) with the same content, a lower level and no mail.
func TestFindOnAccount(t *testing.T) {
	fresh := fixtures.At(10 * time.Minute)
	main := fixtures.Forever(t)
	alt := fixtures.Forever(t)
	c := snapshot.AsMap(alt["character"])
	c["level"], c["moneyCopper"], c["class"] = int64(12), int64(10000), "Mage"
	alt["mail"] = nil
	db := &snapshot.Database{Last: "Piekasz", Characters: map[string]snapshot.Snapshot{"Piekasz": main, "Altpiekasz": alt}}

	found, err := FindOnAccount(db, FindQuery{Item: "linen"}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	// On each character: 14 in bags, 20 in the bank; on the main one another 20 in mail.
	if found["total"] != int64(88) {
		t.Errorf("Linen Cloth total: %s", fixtures.JSON(t, found))
	}
	fixtures.WantAll(t, "search", fixtures.JSON(t, found), `"where":"mail"`, `"where":"bank"`, `"character":"Altpiekasz"`)
	if f, _ := FindOnAccount(db, FindQuery{Item: "2515", Character: "Piekasz"}, fresh); !strings.Contains(fixtures.JSON(t, f), `"slot":"ammo"`) {
		t.Errorf("ammo by ID: %s", fixtures.JSON(t, f))
	}
	if _, err := FindOnAccount(db, FindQuery{Item: "x", Character: "Nobody"}, fresh); err == nil {
		t.Error("an unknown character should give an error")
	}
}

func TestSummarizeBagsAcceptsFloatSizes(t *testing.T) {
	s := fixtures.Forever(t)
	for _, raw := range snapshot.AsList(s["bags"]) {
		b := snapshot.AsMap(raw)
		size, _ := snapshot.AsInt(b["size"])
		free, _ := snapshot.AsInt(b["free"])
		b["size"], b["free"] = float64(size), float64(free) // this is how dumps come back from JSON history
	}
	out := map[string]any{}
	SummarizeBags(s, out)
	want := BagSpace(fixtures.Forever(t))
	if out["totalSlots"] != want["totalSlots"] || out["freeSlots"] != want["freeSlots"] || out["totalSlots"].(int64) == 0 {
		t.Errorf("bag totals with floating-point numbers: %v, want %v", out, want)
	}
	if out["ammo"] == nil {
		t.Error("the bag summary should include ammo")
	}
}

// The real dump stores DPS in stats under the key "damage per second"; it must not be counted twice alongside the DPS from the tooltip.
func TestItemNumbersFoldsDamagePerSecondStat(t *testing.T) {
	item := map[string]any{
		"stats":   map[string]any{"damage per second": 13.6, "Stamina": int64(5)},
		"tooltip": []any{"(13.6 damage per second)"},
	}
	nums := itemNumbers(item)
	if nums["DPS"] != 13.6 || nums["Stamina"] != 5 {
		t.Errorf("numbers: %v", nums)
	}
	if _, dup := nums["damage per second"]; dup {
		t.Errorf("DPS counted twice: %v", nums)
	}
}

func TestCountHoldings(t *testing.T) {
	s := fixtures.Forever(t)
	h := Count(s, "sharp arrow") // equipped ammo in the ammo slot
	if h.Equipped != 812 || h.Bags != 200 || h.Total() != 1012 {
		t.Errorf("Count(Sharp Arrow) = %+v", h)
	}
	if got := Count(s, "Zzzz"); got.Total() != 0 {
		t.Errorf("missing item: %+v", got)
	}
}

func TestStatTokensGiveEnglishNamesOnAnyClient(t *testing.T) {
	german := map[string]any{
		"stats":      map[string]any{"Beweglichkeit": int64(3), "Schaden pro Sekunde": 8.4, "Rüstung": int64(40)},
		"statTokens": map[string]any{"ITEM_MOD_AGILITY_SHORT": int64(3), "ITEM_MOD_DAMAGE_PER_SECOND_SHORT": 8.4, "RESISTANCE0_NAME": int64(40), "ITEM_MOD_HIT_RATING_SHORT": int64(1)},
		"tooltip":    []any{"Schwert", "9 - 18 Schaden | Tempo 1,60"},
	}
	english := map[string]any{
		"stats":   map[string]any{"Agility": int64(3), "damage per second": 8.4, "Armor": int64(40), "Hit Rating": int64(1)},
		"tooltip": []any{"Sword", "9 - 18 Damage | Speed 1.60"},
	}
	de, en := itemNumbers(german), itemNumbers(english)
	for _, k := range []string{"Agility", "DPS", "Armor", "Hit Rating"} {
		if de[k] != en[k] || de[k] == 0 {
			t.Errorf("%s: German %v, English %v", k, de[k], en[k])
		}
	}
	if _, ok := de["Beweglichkeit"]; ok {
		t.Error("the German label must not become a stat name")
	}
}

func TestWeaponNumbersWithoutEnglishTooltip(t *testing.T) {
	// Addon 0.10.0: speed, damage range and DPS are numbers; the German tooltip ("Tempo 1,60") is not parsed at all.
	german := map[string]any{
		"tooltip": []any{"Dolch", "9 - 18 Schaden | Tempo 1,60"}, "speed": 1.6, "dps": 8.4, "damageMin": int64(9), "damageMax": int64(18),
		"statTokens": map[string]any{"ITEM_MOD_DAMAGE_PER_SECOND_SHORT": 8.4},
	}
	nums := itemNumbers(german)
	if nums["Speed"] != 1.6 || nums["DPS"] != 8.4 {
		t.Errorf("numbers from the numeric fields: %v", nums)
	}
	if readsEnglishTooltip(map[string]any{"classID": int64(2), "tooltip": german["tooltip"], "speed": 1.6, "statTokens": german["statTokens"]}) {
		t.Error("a weapon with a numeric speed and stat tokens needs no English tooltip")
	}
	// A numeric DPS wins over a (different) number in the tooltip text; without numeric fields the English text is the fallback.
	both := map[string]any{"dps": 10.0, "tooltip": []any{"(9.5 damage per second)"}}
	if got := itemNumbers(both)["DPS"]; got != 10 {
		t.Errorf("numeric DPS = %v", got)
	}
	old := map[string]any{"classID": int64(2), "tooltip": []any{"9 - 18 Damage | Speed 1.60"}}
	if n := itemNumbers(old); n["Speed"] != 1.6 || n["DPS"] != 8.4 {
		t.Errorf("fallback for an old dump: %v", n)
	}
	if !readsEnglishTooltip(old) {
		t.Error("an old dump's weapon reads the English tooltip")
	}
	// The German test dump: the dagger has numeric fields, so the comparison does not depend on text.
	dagger := snapshot.EquippedBySlot(fixtures.Dede(t).Equipment())["mainHand"]
	if n := itemNumbers(dagger); n["Speed"] != 1.6 || n["DPS"] != 8.4 {
		t.Errorf("the dede dump's dagger: %v (%v)", n, dagger)
	}
	if _, ok := dagger["speed"]; !ok || readsEnglishTooltip(dagger) {
		t.Errorf("dede dagger: numeric speed expected, English tooltip not needed: %v", dagger)
	}
}
