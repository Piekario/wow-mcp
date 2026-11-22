// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestBuybackInVendorView(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := fixtures.At(90 * time.Minute)
	out := Vendor(s, VendorQuery{}, fresh)
	b, ok := out["buyback"].(map[string]any)
	if !ok {
		t.Fatalf("no buyback in the offer: %v", out)
	}
	text := fixtures.JSON(t, b)
	fixtures.WantAll(t, "buyback", text, `"npc":"Einris Brightspear"`, "Linen Cloth", `"count":2`, `"capturedAge"`, `"assumption"`, `"canAfford"`)

	only := Vendor(s, VendorQuery{Query: "skinning"}, fresh)["buyback"].(map[string]any)
	if only["count"] != 1 {
		t.Errorf("filter by name: %v", only["count"])
	}
	if _, has := Vendor(s, VendorQuery{Npc: "does not exist"}, fresh)["buyback"]; has {
		t.Error("an unknown NPC has no buyback")
	}
}

func TestBuybackAbsent(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s, "buyback")
	if Buyback(s, "", fixtures.RealTime()) != nil {
		t.Error("nil without the section")
	}
	if _, has := Vendor(s, VendorQuery{}, fixtures.RealTime())["buyback"]; has {
		t.Error("offer without buyback")
	}
}

func TestBuybackAffordability(t *testing.T) {
	s := snapshot.Snapshot{
		"character": map[string]any{"moneyCopper": int64(100), "level": int64(10)},
		"buyback": map[string]any{"npc": "X", "items": []any{
			map[string]any{"name": "A", "count": int64(1), "priceCopper": int64(50)},
			map[string]any{"name": "B", "count": int64(5), "priceCopper": int64(500)},
		}},
	}
	b := Buyback(s, "", snapshot.Freshness{})
	items := b["items"].([]any)
	if items[0].(map[string]any)["canAfford"] != true || items[1].(map[string]any)["canAfford"] != false {
		t.Errorf("canAfford: %v", items)
	}
	if b["totalPrice"] == nil {
		t.Error("totalPrice")
	}
}
