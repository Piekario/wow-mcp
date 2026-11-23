// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package makeorbuy

import (
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func reagent(name string, need int64) map[string]any {
	return map[string]any{"name": name, "need": need}
}

func ah(id, buyout, seenAt int64) map[string]any {
	return map[string]any{"id": id, "minBuyout": buyout, "seenAt": seenAt}
}

func testSnap() snapshot.Snapshot {
	now := int64(fixtures.ForeverDumpedAt)
	return snapshot.Snapshot{
		"recipes": map[string]any{"Leatherworking": map[string]any{"recipes": []any{
			map[string]any{"name": "Embossed Gloves", "itemId": int64(4239), "reagents": []any{reagent("Light Leather", 3), reagent("Coarse Thread", 2)}},
			map[string]any{"name": "Cheap Kit", "reagents": []any{reagent("Light Leather", 1)}},
			map[string]any{"name": "Mystery Boots", "reagents": []any{reagent("Light Leather", 1), reagent("Unpriced Dust", 1)}},
			map[string]any{"name": "Old Belt", "reagents": []any{reagent("Light Leather", 1)}},
			map[string]any{"name": "Growl"},
		}}},
		"auctions": map[string]any{"prices": map[string]any{
			"Light Leather":   ah(2318, 100, now),
			"Embossed Gloves": ah(4239, 900, now),
			"Cheap Kit":       ah(1, 50, now),
			"Mystery Boots":   ah(2, 5000, now),
			"Old Belt":        ah(3, 500, now-72*3600),
		}},
		"vendor": map[string]any{"npc": "Thread Vendor", "capturedAt": now, "items": []any{
			map[string]any{"name": "Coarse Thread", "priceCopper": int64(160), "stack": int64(2)},
		}},
	}
}

func byName(out map[string]any, name string) map[string]any {
	for _, raw := range snapshot.AsList(out["recipes"]) {
		if r := snapshot.AsMap(raw); r["name"] == name {
			return r
		}
	}
	return nil
}

func TestView(t *testing.T) {
	out := View(testSnap(), Query{}, fixtures.At(0))
	g := byName(out, "Embossed Gloves")
	// 3*100 + 2*80 (vendor 160 for 2 units) = 460; product 900.
	if g["reagentCostCopper"] != int64(460) || g["marginCopper"] != int64(440) || g["verdict"] != VerdictMake {
		t.Errorf("Gloves: %v", g)
	}
	if g["marginAfterCut"] != snapshot.Money(900*95/100-460).Signed() {
		t.Errorf("marginAfterCut: %v", g["marginAfterCut"])
	}
	kit := byName(out, "Cheap Kit")
	if kit["verdict"] != VerdictBuy || kit["marginCopper"] != int64(-50) {
		t.Errorf("Cheap Kit: %v", kit)
	}
	boots := byName(out, "Mystery Boots")
	if boots["verdict"] != VerdictUnknown || boots["reagentCost"] != nil || boots["margin"] != nil {
		t.Errorf("a missing price must not be zero: %v", boots)
	}
	if m := boots["missingPrices"].([]string); len(m) != 1 || m[0] != "Unpriced Dust" {
		t.Errorf("missingPrices: %v", m)
	}
	if !strings.Contains(boots["knownPartOfCost"].(string), "incomplete") {
		t.Errorf("knownPartOfCost: %v", boots["knownPartOfCost"])
	}
	belt := byName(out, "Old Belt")
	if st, _ := belt["stalePrices"].([]string); len(st) != 1 || !strings.Contains(st[0], "finished item") {
		t.Errorf("stalePrices: %v", belt["stalePrices"])
	}
	if byName(out, "Growl") != nil {
		t.Error("a recipe without reagents is skipped")
	}
	// Sorting by margin: known first, unknown last.
	list := snapshot.AsList(out["recipes"])
	if snapshot.AsMap(list[0])["name"] != "Embossed Gloves" || snapshot.AsMap(list[len(list)-1])["verdict"] != VerdictUnknown {
		t.Errorf("order: %v", fixtures.JSON(t, out["recipes"]))
	}
	counts := out["byVerdict"].(map[string]int)
	if counts[VerdictMake] != 2 || counts[VerdictBuy] != 1 || counts[VerdictUnknown] != 1 {
		t.Errorf("byVerdict: %v", counts)
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name  string
		q     Query
		first string
		count int
	}{
		{"query", Query{Query: "gloves"}, "Embossed Gloves", 1},
		{"sort by name", Query{Sort: "name"}, "Cheap Kit", 4},
		{"limit", Query{Limit: 2}, "Embossed Gloves", 2},
		{"profession with no match", Query{Profession: "Tailoring"}, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := View(testSnap(), tt.q, fixtures.At(0))
			list := snapshot.AsList(out["recipes"])
			if len(list) != tt.count {
				t.Fatalf("count %d, wanted %d", len(list), tt.count)
			}
			if tt.count > 0 && snapshot.AsMap(list[0])["name"] != tt.first {
				t.Errorf("first: %v", snapshot.AsMap(list[0])["name"])
			}
		})
	}
}

func TestMissingSections(t *testing.T) {
	out := View(snapshot.Snapshot{}, Query{}, fixtures.At(0))
	if !strings.Contains(out["note"].(string), "/mcp reload") {
		t.Errorf("no recipes: %v", out)
	}
	s := testSnap()
	delete(s, "auctions")
	delete(s, "vendor")
	out = View(s, Query{}, fixtures.At(0))
	if out["hint"] == nil {
		t.Error("no auction prices = hint")
	}
	for _, raw := range snapshot.AsList(out["recipes"]) {
		if snapshot.AsMap(raw)["verdict"] != VerdictUnknown {
			t.Errorf("without prices everything is unknown: %v", raw)
		}
	}
	// Broken types do not cause a panic.
	View(snapshot.Snapshot{"recipes": map[string]any{"X": "bad"}, "auctions": []any{}}, Query{}, fixtures.At(0))
}

func TestAnalysePricesCraftsWithoutZeroForMissingPrices(t *testing.T) {
	r := Analyse(testSnap(), Query{}, fixtures.At(0))
	byName := map[string]Evaluation{}
	for _, e := range r.Evaluations {
		byName[e.Name] = e
	}
	gloves := byName["Embossed Gloves"]
	if !gloves.Known() || gloves.Cost != 460 || gloves.Margin != 440 || gloves.Verdict() != VerdictMake {
		t.Errorf("gloves = %+v", gloves)
	}
	boots := byName["Mystery Boots"]
	if boots.Known() || boots.Verdict() != VerdictUnknown || len(boots.Missing) != 1 || boots.Missing[0].Name != "Unpriced Dust" {
		t.Errorf("boots = %+v", boots)
	}
	if len(byName["Old Belt"].Stale) != 1 || !byName["Old Belt"].Stale[0].Finished {
		t.Errorf("belt stale = %+v", byName["Old Belt"].Stale)
	}
	if r.Total != 4 || r.ByVerdict[VerdictMake] != 2 || r.Truncated() {
		t.Errorf("report: total %d, %v", r.Total, r.ByVerdict)
	}
}
