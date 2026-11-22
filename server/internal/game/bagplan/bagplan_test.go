// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package bagplan

import (
	"maps"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

// actions maps an item name to the action in the response.
func actions(t *testing.T, out map[string]any) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, raw := range out["items"].([]any) {
		e := raw.(map[string]any)
		got[e["name"].(string)] = e["action"].(string)
		if e["reason"] == "" {
			t.Errorf("%v: no reason", e["name"])
		}
	}
	return got
}

func TestRealBags(t *testing.T) {
	out := View(fixtures.Real(t), fixtures.At(0))
	got := actions(t, out)
	want := map[string]string{
		"Sharp Arrow":                    Keep, // bow -> arrows
		"Blump Family Fishing Pole":      Keep, // the character has Fishing
		"Mining Pick":                    Keep,
		"Skinning Knife":                 Keep,
		"Blood Sausage":                  Keep, // food, pet
		"Scroll of Agility":              Keep, // consumable
		"Hearthstone":                    Keep, // price 0
		"Parker's Lunch":                 Keep, // quest from the log (A Free Lunch)
		"An Unsent Letter":               Bank, // quest item without a quest: never to the trash
		"Coarse Stone":                   Bank, // material
		"Pattern: Rugged Leather Pants":  Bank, // green without an auction price
		"Pattern: Totemic Leather Armor": Bank, // blue without an auction price
	}
	for name, action := range want {
		if got[name] != action {
			t.Errorf("%s: %q, want %q", name, got[name], action)
		}
	}
	for name, a := range got {
		if a != Keep && a != Bank && a != Sell && a != Auction {
			t.Errorf("%s: disallowed action %q", name, a)
		}
	}
	for _, bad := range []string{"destroy", "delete", "discard", "drop\""} {
		if strings.Contains(strings.ToLower(fixtures.JSON(t, out["items"])), `"action":"`+bad) {
			t.Errorf("the plan advises %q", bad)
		}
	}
	space := out["bagSlots"].(map[string]any)
	if space["free"] != int64(14) {
		t.Errorf("free slots: %v", space)
	}
	if out["bankNote"] == nil {
		t.Error("the real dump has no bank: expected a bank note")
	}
}

func addBagItem(s snapshot.Snapshot, item map[string]any) {
	bag := snapshot.AsMap(snapshot.AsList(s["bags"])[0])
	bag["items"] = append(snapshot.AsList(bag["items"]), item)
}

func TestSellAuctionAndUpgrade(t *testing.T) {
	s := fixtures.Forever(t)
	addBagItem(s, map[string]any{"name": "Broken Fang", "id": int64(1), "count": int64(3), "quality": int64(0), "sellPrice": int64(10), "type": "Miscellaneous", "subType": "Junk"})
	addBagItem(s, map[string]any{"name": "White Shirt", "id": int64(2), "count": int64(1), "quality": int64(1), "sellPrice": int64(40), "type": "Armor", "subType": "Cloth", "equipLoc": "INVTYPE_BODY", "itemLevel": int64(1), "requiredLevel": int64(1)})
	addBagItem(s, map[string]any{"name": "Valuable Material", "id": int64(3), "count": int64(2), "quality": int64(1), "sellPrice": int64(10), "type": "Miscellaneous", "subType": "Other"})
	s["auctions"] = map[string]any{"prices": map[string]any{"Valuable Material": map[string]any{"name": "Valuable Material", "minBuyout": int64(5000), "seenAt": int64(fixtures.ForeverDumpedAt)}}}
	out := View(s, fixtures.At(0))
	got := actions(t, out)
	if got["Broken Fang"] != Sell {
		t.Errorf("grey: %q", got["Broken Fang"])
	}
	if got["Valuable Material"] != Auction {
		t.Errorf("auction price: %q", got["Valuable Material"])
	}
	if got["White Shirt"] != Keep && got["White Shirt"] != Sell {
		t.Errorf("white shirt: %q", got["White Shirt"])
	}
	summary := out["summary"].(map[string]any)
	if sell := summary[Sell].(map[string]any); sell["vendorValue"] == nil {
		t.Errorf("sell summary: %v", sell)
	}
	if auction := summary[Auction].(map[string]any); auction["afterCut"] != "95s 0c" {
		t.Errorf("auction summary: %v", auction)
	}
	// Grey and auction items free slots (sell + auction + bank).
	if freed, _ := out["slotsFreed"].(int64); freed < 2 {
		t.Errorf("slotsFreed = %v", out["slotsFreed"])
	}
	// Sorted: sell, auction, bank, keep.
	var last = -1
	for _, raw := range out["items"].([]any) {
		r := rank(raw.(map[string]any)["action"].(string))
		if r < last {
			t.Fatalf("wrong order: %v", out["items"])
		}
		last = r
	}
}

func TestWrongAmmoGoesToBankNotSell(t *testing.T) {
	// An old dump (English text only) and a 0.9.0 dump from a German client (IDs, German text): the same decision.
	for _, tc := range []struct {
		name        string
		bow, bullet map[string]any
	}{
		{"english text", map[string]any{"subType": "Bows"}, map[string]any{"type": "Projectile", "subType": "Bullet"}},
		{"ids, german text",
			map[string]any{"subType": "Bögen", "classID": int64(2), "subclassID": int64(2)},
			map[string]any{"type": "Projektil", "subType": "Kugel", "classID": int64(6), "subclassID": int64(3)}},
	} {
		s := fixtures.Forever(t)
		ranged := snapshot.EquippedBySlot(s.Equipment())["ranged"]
		delete(ranged, "classID")
		delete(ranged, "subclassID")
		maps.Copy(ranged, tc.bow)
		bullet := map[string]any{"name": "Rough Bullet", "id": int64(9), "count": int64(100), "quality": int64(1), "sellPrice": int64(1)}
		maps.Copy(bullet, tc.bullet)
		addBagItem(s, bullet)
		got := actions(t, View(s, fixtures.At(0)))
		if got["Rough Bullet"] != Bank {
			t.Errorf("%s: bullets for a bow: %q (want bank, not sell)", tc.name, got["Rough Bullet"])
		}
	}
}

func TestEmptyBags(t *testing.T) {
	s := fixtures.Forever(t)
	s["bags"] = []any{}
	if out := View(s, fixtures.At(0)); out["note"] == nil {
		t.Errorf("empty bags: %v", out)
	}
}
