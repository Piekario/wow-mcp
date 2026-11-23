// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestInspectBankTyped(t *testing.T) {
	r := InspectBank(fixtures.Forever(t), BankQuery{Sellable: true, Type: "trade goods"})
	if !r.Saved || len(r.Entries) != 1 || r.TotalValue != 260 || r.FreeSlots != 23 {
		t.Errorf("bank: %+v", r)
	}
	if InspectBank(snapshot.Snapshot{}, BankQuery{}).Saved {
		t.Error("no bank section, nothing saved")
	}
}

func TestInspectBagsFindsAmmo(t *testing.T) {
	r := InspectBags(fixtures.Forever(t))
	if r.Ammo == nil || r.Ammo.Name != "Sharp Arrow" || r.Ammo.Count != 812 || r.Total == 0 {
		t.Errorf("bags: %+v", r)
	}
}

func TestInspectVendorUnknownNpcAndAffordable(t *testing.T) {
	s := fixtures.Forever(t)
	if r := InspectVendor(s, VendorQuery{Npc: "Nobody Here"}); r.UnknownNpc != "Nobody Here" || r.Saved {
		t.Errorf("unknown vendor: %+v", r)
	}
	r := InspectVendor(s, VendorQuery{Affordable: true})
	if !r.Saved || len(r.Offers) != 1 || !r.Offers[0].Offer.CanAfford {
		t.Errorf("affordable offer: %+v", r.Offers)
	}
}

func TestDistinctByNameKeepsLastSource(t *testing.T) {
	a := Item{raw: record{"name": "Bow"}}
	b := Item{raw: record{"name": "Arrow"}}
	got := distinctByName([]FoundItem{{a, InBags}, {b, InBags}, {a, AtVendor}})
	if len(got) != 2 || got[0].Source != AtVendor {
		t.Errorf("distinct: %+v", got)
	}
}

func TestStockOfSkipsUnkept(t *testing.T) {
	s := fixtures.Forever(t)
	all := StockOf(s, nil)
	linen := all["linen cloth"]
	if linen == nil || linen.Bags != 14 || linen.Bank != 20 || linen.Total() != 34 {
		t.Fatalf("linen: %+v", linen)
	}
	none := StockOf(s, func(it Item) bool { return it.Name != "Linen Cloth" })
	if none["linen cloth"] != nil {
		t.Error("keep=false skips the item")
	}
}
