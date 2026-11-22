// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"testing"

	"wowmcp/internal/fixtures"
)

func TestSellersOfAndCost(t *testing.T) {
	s := fixtures.Forever(t)
	got := SellersOf(s, "sharp arrow")
	if len(got) < 2 {
		t.Fatalf("at least two remembered vendors sell Sharp Arrow: %+v", got)
	}
	cheap := got[0]
	if cheap.Stack != 200 || cheap.PriceCopper != 300 || cheap.UnitCopper != 1.5 {
		t.Errorf("vendor: %+v", cheap)
	}
	if packs, cost := cheap.CostFor(250); packs != 2 || cost != 600 {
		t.Errorf("250 units = 2 packs of 300c: %d %d", packs, cost)
	}
	if packs, cost := cheap.CostFor(0); packs != 0 || cost != 0 {
		t.Error("zero units cost nothing")
	}
	if len(SellersOf(s, "No such item")) != 0 {
		t.Error("unknown item")
	}
	if d := cheap.Describe(); d["unitPrice"] != "1.5 c" {
		t.Errorf("description: %v", d)
	}
}

func TestVendorKnownAndNpc(t *testing.T) {
	s := fixtures.Forever(t)
	out := Vendor(s, VendorQuery{}, fixtures.At(0))
	known, _ := out["known"].([]string)
	if len(known) < 2 {
		t.Fatalf("known: %v", out["known"])
	}
	other := Vendor(s, VendorQuery{Npc: "laird"}, fixtures.At(0))
	if other["npc"] != "Laird" {
		t.Errorf("selected vendor: %v", other["npc"])
	}
	missing := Vendor(s, VendorQuery{Npc: "Zzz"}, fixtures.At(0))
	if missing["note"] == nil || missing["known"] == nil {
		t.Errorf("unknown vendor: %v", missing)
	}
}
