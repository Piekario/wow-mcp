// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package reagents

import (
	"path/filepath"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

const realChar = "Piekasz"

func setItemCount(s snapshot.Snapshot, name string, n int64) {
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			if item := snapshot.AsMap(raw); item["name"] == name {
				item["count"] = n
			}
		}
	}
}

// realHistory: the real dump as current, and two earlier ones (every hour of play) with more water.
func realHistory(t *testing.T) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	h := &storage.History{Dir: filepath.Join(t.TempDir(), "h"), Keep: 50}
	cur := fixtures.Real(t)
	played, _ := snapshot.AsInt(cur.Map("played")["totalSeconds"])
	for i, water := range []int64{30, 20} {
		s := fixtures.Real(t)
		back := int64(2 - i)
		s["dumpedAt"] = int64(fixtures.RealDumpedAt) - back*3600
		snapshot.AsMap(s["played"])["totalSeconds"] = played - back*3600
		setItemCount(s, "Refreshing Spring Water", water)
		if err := h.Save(realChar, s); err != nil {
			t.Fatal(err)
		}
	}
	return progress.Tracker{History: h}, cur
}

func entries(out map[string]any) map[string]map[string]any {
	got := map[string]map[string]any{}
	for _, c := range out["categories"].([]any) {
		for _, raw := range c.(map[string]any)["items"].([]any) {
			e := raw.(map[string]any)
			got[e["name"].(string)] = e
		}
	}
	return got
}

func TestCategoriesOnRealBags(t *testing.T) {
	out := View(realChar, fixtures.Real(t), progress.Tracker{})
	cats := map[string]bool{}
	for _, c := range out["categories"].([]any) {
		cats[c.(map[string]any)["category"].(string)] = true
	}
	for _, want := range []string{"ammo", "food", "otherConsumables", "reagents"} {
		if !cats[want] {
			t.Errorf("missing category %s: %v", want, cats)
		}
	}
	got := entries(out)
	if got["Sharp Arrow"]["total"] != int64(2124) || got["Refreshing Spring Water"]["total"] != int64(10) {
		t.Errorf("stocks: %v %v", got["Sharp Arrow"], got["Refreshing Spring Water"])
	}
	if _, ok := got["Hearthstone"]; ok {
		t.Error("Hearthstone is not consumable")
	}
	if _, ok := got["Coarse Stone"]; ok {
		t.Error("a crafting material is not a reagent")
	}
	// Without history: just the stock and an explanation.
	if c := out["consumption"].(map[string]any); c["note"] == nil {
		t.Errorf("without history a note is expected: %v", c)
	}
	if out["petFood"] == nil {
		t.Error("the character has a pet: petFood expected")
	}
}

func TestConsumptionFromHistory(t *testing.T) {
	tr, cur := realHistory(t)
	out := View(realChar, cur, tr)
	water := entries(out)["Refreshing Spring Water"]
	// 30 -> 20 -> 10: 20 pieces in 2 h = 10/h, a stock of 10 lasts about 1 h.
	if water["perHour"] != 10.0 || water["hoursLeft"] != 1.0 || water["usedInWindow"] != int64(20) {
		t.Errorf("water: %v", water)
	}
	arrows := entries(out)["Sharp Arrow"]
	if arrows["perHour"] != 0.0 || arrows["hoursLeft"] != nil {
		t.Errorf("arrows do not decrease: %v", arrows)
	}
	window := out["consumption"].(map[string]any)
	if window["window"] != "current session" || window["playHours"] != 2.0 {
		t.Errorf("window: %v", window)
	}
	if out["lowStock"] != nil {
		t.Errorf("1.5 h is not low stock yet: %v", out["lowStock"])
	}
}

func TestLowStockFlag(t *testing.T) {
	tr, cur := realHistory(t)
	setItemCount(cur, "Refreshing Spring Water", 4) // 4 / 8.5 h < 1 h; a drop 20 -> 4 = 16 in the second hour
	out := View(realChar, cur, tr)
	low, _ := out["lowStock"].([]string)
	if len(low) != 1 || low[0] != "Refreshing Spring Water" {
		t.Errorf("lowStock = %v", out["lowStock"])
	}
}

func TestNothingConsumable(t *testing.T) {
	s := fixtures.Real(t)
	s["bags"] = []any{}
	s["equipment"] = []any{}
	if out := View(realChar, s, progress.Tracker{}); out["note"] == nil || out["categories"] != nil {
		t.Errorf("no consumables: %v", out)
	}
}

func TestShortagesSkipAmmoAndPlentiful(t *testing.T) {
	use := func(perHour float64) consumption.Usage { return consumption.Usage{PerHour: perHour} }
	line := func(name string, bags int64, perHour float64) Line {
		l := Line{Name: name, Holdings: items.Holdings{Bags: bags}, Measured: true, Use: use(perHour)}
		l.HoursLeft, l.HasHoursLeft = l.Use.HoursLeft(l.Total())
		return l
	}
	r := Report{Groups: []Group{
		{category: ammo, Lines: []Line{line("Sharp Arrow", 10, 100)}},
		{category: bandages, Lines: []Line{line("Linen Bandage", 3, 12), line("Silk Bandage", 60, 6), {Name: "No rate", Holdings: items.Holdings{Bags: 5}}}},
	}}
	got := r.Shortages()
	if len(got) != 1 || got[0].Name != "Linen Bandage" || got[0].Category != "bandages" || got[0].Total != 3 || got[0].PerHour != 12 || got[0].HoursLeft != 0.3 {
		t.Errorf("shortages: %+v", got)
	}
	if low := r.LowStock(); len(low) != 2 || low[0] != "Linen Bandage" || low[1] != "Sharp Arrow" {
		t.Errorf("low stock keeps ammo and sorts by name: %v", low)
	}
	if len(Report{}.Shortages()) != 0 {
		t.Error("empty report")
	}
}
